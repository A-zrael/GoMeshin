package mesh

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	meshtastic "gitplac.si/gomeshtastic/protobuf/v2"
	"google.golang.org/protobuf/proto"
	"meshin/meshtasticapi"
)

type Config struct {
	Port  string
	Baud  int
	Store Store
}

type Mesh struct {
	radio *meshtasticapi.Client
	store Store

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu       sync.RWMutex
	myNode   uint32
	nodes    map[uint32]Node
	channels map[int32]Channel

	subsMu      sync.RWMutex
	subscribers map[chan Message]struct{}

	traceMu sync.Mutex
	traces  map[uint32]chan TraceRoute
}

type Message struct {
	ID         uint32
	From       NodeRef
	To         uint32
	Channel    ChannelRef
	Text       string
	RSSI       int32
	SNR        float32
	ReceivedAt time.Time
}

type TraceRoute struct {
	RequestID uint32
	From      uint32
	To        uint32
	Towards   []TraceHop
	Back      []TraceHop
}

type TraceHop struct {
	Node NodeRef
	SNR  *float32
}

type Node struct {
	Num       uint32
	ID        string
	LongName  string
	ShortName string
	LastSeen  time.Time
}

type NodeRef struct {
	Num       uint32
	ID        string
	LongName  string
	ShortName string
}

type Channel struct {
	Index           int32
	Name            string
	Role            string
	ID              uint32
	PSKBytes        int
	UplinkEnabled   bool
	DownlinkEnabled bool
}

type ChannelRef struct {
	Index int32
	Name  string
}

type SendOptions struct {
	Channel string
	To      uint32
	WantAck bool
}

type ChannelOptions struct {
	Name string
	PSK  []byte
}

type TraceRouteOptions struct {
	To       uint32
	Channel  string
	HopLimit uint32
}

func Open(ctx context.Context, cfg Config) (*Mesh, error) {
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore()
	}

	radio, err := meshtasticapi.Open(ctx, meshtasticapi.Config{
		Port: cfg.Port,
		Baud: cfg.Baud,
	})
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	m := &Mesh{
		radio:       radio,
		store:       cfg.Store,
		ctx:         runCtx,
		cancel:      cancel,
		done:        make(chan struct{}),
		nodes:       make(map[uint32]Node),
		channels:    make(map[int32]Channel),
		subscribers: make(map[chan Message]struct{}),
		traces:      make(map[uint32]chan TraceRoute),
	}

	for _, channel := range radio.Channels() {
		m.upsertChannel(convertChannel(channel))
	}
	for _, node := range radio.Nodes() {
		m.upsertNode(Node{
			Num:       node.Num,
			ID:        node.ID,
			LongName:  node.LongName,
			ShortName: node.ShortName,
			LastSeen:  time.Now(),
		})
	}
	if myNode := radio.MyNode(); myNode != nil {
		m.myNode = myNode.Num
	}

	go m.run()

	return m, nil
}

func (m *Mesh) Close() error {
	m.cancel()
	err := m.radio.Close()
	<-m.done
	return err
}

func (m *Mesh) Send(text string, opts SendOptions) (uint32, error) {
	channelIndex, err := m.resolveChannel(opts.Channel)
	if err != nil {
		return 0, err
	}

	return m.radio.SendText(text, meshtasticapi.SendOptions{
		To:      opts.To,
		Channel: uint32(channelIndex),
		WantAck: opts.WantAck,
	})
}

func (m *Mesh) AddChannel(ctx context.Context, opts ChannelOptions) error {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return errors.New("channel name is required")
	}

	m.mu.RLock()
	for _, channel := range m.channels {
		if channel.Name == name {
			m.mu.RUnlock()
			return fmt.Errorf("channel %q already exists", name)
		}
	}
	index := m.firstFreeChannelIndexLocked()
	myNode := m.myNode
	m.mu.RUnlock()

	if index == 0 {
		return errors.New("no free secondary channel slot")
	}
	if myNode == 0 {
		return errors.New("local node number is not known yet")
	}

	psk := opts.PSK
	if len(psk) == 0 {
		psk = []byte{1}
	}

	channel := &meshtastic.Channel{
		Index: index,
		Role:  meshtastic.Channel_SECONDARY,
		Settings: &meshtastic.ChannelSettings{
			Name: name,
			Psk:  psk,
		},
	}

	if _, err := m.sendAdmin(ctx, myNode, &meshtastic.AdminMessage{
		PayloadVariant: &meshtastic.AdminMessage_SetChannel{
			SetChannel: channel,
		},
	}); err != nil {
		return err
	}

	m.upsertChannel(convertChannel(meshtasticapi.Channel{
		Index:    index,
		Name:     name,
		Role:     meshtastic.Channel_SECONDARY,
		PSKBytes: len(psk),
	}))
	return nil
}

func (m *Mesh) RemoveChannel(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "Primary" {
		return errors.New("only named secondary channels can be removed")
	}

	m.mu.RLock()
	var target Channel
	found := false
	for _, channel := range m.channels {
		if channel.Name == name {
			target = channel
			found = true
			break
		}
	}
	myNode := m.myNode
	m.mu.RUnlock()

	if !found {
		return fmt.Errorf("channel %q not found", name)
	}
	if target.Index == 0 {
		return errors.New("primary channel cannot be removed")
	}
	if myNode == 0 {
		return errors.New("local node number is not known yet")
	}

	if _, err := m.sendAdmin(ctx, myNode, &meshtastic.AdminMessage{
		PayloadVariant: &meshtastic.AdminMessage_SetChannel{
			SetChannel: &meshtastic.Channel{
				Index: target.Index,
				Role:  meshtastic.Channel_DISABLED,
			},
		},
	}); err != nil {
		return err
	}

	m.upsertChannel(Channel{
		Index: target.Index,
		Role:  meshtastic.Channel_DISABLED.String(),
	})
	return nil
}

func (m *Mesh) TraceRoute(ctx context.Context, opts TraceRouteOptions) (TraceRoute, error) {
	if opts.To == 0 {
		return TraceRoute{}, errors.New("destination node is required")
	}

	channelIndex, err := m.resolveChannel(opts.Channel)
	if err != nil {
		return TraceRoute{}, err
	}

	payload, err := proto.Marshal(&meshtastic.RouteDiscovery{})
	if err != nil {
		return TraceRoute{}, err
	}

	done := make(chan struct {
		id  uint32
		err error
	}, 1)
	go func() {
		id, err := m.radio.SendData(meshtasticapi.SendOptions{
			To:       opts.To,
			Channel:  uint32(channelIndex),
			Port:     meshtastic.PortNum_TRACEROUTE_APP,
			Payload:  payload,
			WantResp: true,
			HopLimit: opts.HopLimit,
		})
		done <- struct {
			id  uint32
			err error
		}{id: id, err: err}
	}()

	select {
	case <-ctx.Done():
		return TraceRoute{}, ctx.Err()
	case result := <-done:
		if result.err != nil {
			return TraceRoute{}, result.err
		}

		reply := make(chan TraceRoute, 1)
		m.traceMu.Lock()
		m.traces[result.id] = reply
		m.traceMu.Unlock()
		defer m.forgetTrace(result.id)

		select {
		case <-ctx.Done():
			return TraceRoute{RequestID: result.id, To: opts.To}, ctx.Err()
		case trace := <-reply:
			return trace, nil
		}
	}
}

func (m *Mesh) Subscribe(buffer int) (<-chan Message, func()) {
	if buffer < 0 {
		buffer = 0
	}

	ch := make(chan Message, buffer)
	m.subsMu.Lock()
	m.subscribers[ch] = struct{}{}
	m.subsMu.Unlock()

	cancel := func() {
		m.subsMu.Lock()
		if _, ok := m.subscribers[ch]; ok {
			delete(m.subscribers, ch)
			close(ch)
		}
		m.subsMu.Unlock()
	}

	return ch, cancel
}

func (m *Mesh) Messages(ctx context.Context) ([]Message, error) {
	return m.store.Messages(ctx)
}

func (m *Mesh) Nodes(ctx context.Context) ([]Node, error) {
	return m.store.Nodes(ctx)
}

func (m *Mesh) Channels(ctx context.Context) ([]Channel, error) {
	return m.store.Channels(ctx)
}

func (m *Mesh) run() {
	defer close(m.done)

	for {
		select {
		case <-m.ctx.Done():
			m.closeSubscribers()
			return
		case event, ok := <-m.radio.Events():
			if !ok {
				m.closeSubscribers()
				return
			}
			m.handleEvent(event)
		case err, ok := <-m.radio.Errors():
			if ok {
				_ = err
			}
		}
	}
}

func (m *Mesh) handleEvent(event meshtasticapi.Event) {
	switch event.Type {
	case meshtasticapi.EventMyNode:
		if event.MyNode == nil {
			return
		}
		m.mu.Lock()
		m.myNode = event.MyNode.Num
		m.mu.Unlock()
	case meshtasticapi.EventText:
		if event.Packet == nil {
			return
		}
		message := m.convertMessage(*event.Packet)
		_ = m.store.SaveMessage(m.ctx, message)
		m.publish(message)
	case meshtasticapi.EventNode:
		if event.Node == nil {
			return
		}
		node := Node{
			Num:       event.Node.Num,
			ID:        event.Node.ID,
			LongName:  event.Node.LongName,
			ShortName: event.Node.ShortName,
			LastSeen:  time.Now(),
		}
		m.upsertNode(node)
	case meshtasticapi.EventChannel:
		if event.Channel == nil {
			return
		}
		m.upsertChannel(convertChannel(*event.Channel))
	case meshtasticapi.EventTraceRoute:
		if event.TraceRoute == nil {
			return
		}
		m.resolveTrace(convertTraceRoute(*event.TraceRoute, m.nodeRef))
	}
}

func (m *Mesh) sendAdmin(ctx context.Context, to uint32, admin *meshtastic.AdminMessage) (uint32, error) {
	payload, err := proto.Marshal(admin)
	if err != nil {
		return 0, err
	}

	done := make(chan struct {
		id  uint32
		err error
	}, 1)
	go func() {
		id, err := m.radio.SendData(meshtasticapi.SendOptions{
			To:       to,
			Channel:  0,
			Port:     meshtastic.PortNum_ADMIN_APP,
			Payload:  payload,
			WantAck:  true,
			WantResp: false,
			PKI:      true,
		})
		done <- struct {
			id  uint32
			err error
		}{id: id, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case result := <-done:
		return result.id, result.err
	}
}

func (m *Mesh) firstFreeChannelIndexLocked() int32 {
	for index := int32(1); index < 8; index++ {
		channel, ok := m.channels[index]
		if !ok || channel.Role == meshtastic.Channel_DISABLED.String() || channel.Name == "" {
			return index
		}
	}
	return 0
}

func (m *Mesh) convertMessage(packet meshtasticapi.Packet) Message {
	m.mu.RLock()
	node := m.nodes[packet.From]
	channel := m.channels[int32(packet.Channel)]
	m.mu.RUnlock()

	receivedAt := packet.RxTime
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	return Message{
		ID: packet.ID,
		From: NodeRef{
			Num:       packet.From,
			ID:        node.ID,
			LongName:  node.LongName,
			ShortName: node.ShortName,
		},
		To: packet.To,
		Channel: ChannelRef{
			Index: int32(packet.Channel),
			Name:  channel.Name,
		},
		Text:       packet.Text,
		RSSI:       packet.RSSI,
		SNR:        packet.SNR,
		ReceivedAt: receivedAt,
	}
}

func (m *Mesh) upsertNode(node Node) {
	m.mu.Lock()
	m.nodes[node.Num] = node
	m.mu.Unlock()

	_ = m.store.SaveNode(m.ctx, node)
}

func (m *Mesh) upsertChannel(channel Channel) {
	m.mu.Lock()
	m.channels[channel.Index] = channel
	m.mu.Unlock()

	_ = m.store.SaveChannel(m.ctx, channel)
}

func (m *Mesh) publish(message Message) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	for ch := range m.subscribers {
		select {
		case ch <- message:
		default:
		}
	}
}

func (m *Mesh) resolveTrace(route TraceRoute) {
	m.traceMu.Lock()
	reply := m.traces[route.RequestID]
	m.traceMu.Unlock()

	if reply == nil {
		return
	}

	select {
	case reply <- route:
	default:
	}
}

func (m *Mesh) forgetTrace(id uint32) {
	m.traceMu.Lock()
	delete(m.traces, id)
	m.traceMu.Unlock()
}

func (m *Mesh) closeSubscribers() {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	for ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, ch)
	}
}

func (m *Mesh) resolveChannel(name string) (int32, error) {
	if name == "" {
		return 0, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, channel := range m.channels {
		if channel.Name == name {
			return channel.Index, nil
		}
	}

	return 0, fmt.Errorf("channel %q not found", name)
}

func convertChannel(channel meshtasticapi.Channel) Channel {
	name := channel.Name
	if name == "" && channel.Index == 0 {
		name = "Primary"
	}

	return Channel{
		Index:           channel.Index,
		Name:            name,
		Role:            channel.Role.String(),
		ID:              channel.ID,
		PSKBytes:        channel.PSKBytes,
		UplinkEnabled:   channel.UplinkEnabled,
		DownlinkEnabled: channel.DownlinkEnabled,
	}
}

func convertTraceRoute(route meshtasticapi.TraceRoute, lookup func(uint32) NodeRef) TraceRoute {
	return TraceRoute{
		RequestID: route.RequestID,
		From:      route.From,
		To:        route.To,
		Towards:   convertTraceHops(append(append([]uint32{route.To}, route.Route...), route.From), route.SNRTowards, lookup),
		Back:      convertTraceHops(append(append([]uint32{route.From}, route.RouteBack...), route.To), route.SNRBack, lookup),
	}
}

func convertTraceHops(nodes []uint32, snrs []int32, lookup func(uint32) NodeRef) []TraceHop {
	hops := make([]TraceHop, 0, len(nodes))
	for index, nodeNum := range nodes {
		var snr *float32
		if index < len(snrs) && snrs[index] != -128 {
			value := float32(snrs[index]) / 4
			snr = &value
		}
		hops = append(hops, TraceHop{
			Node: lookup(nodeNum),
			SNR:  snr,
		})
	}
	return hops
}

func (m *Mesh) nodeRef(num uint32) NodeRef {
	m.mu.RLock()
	node := m.nodes[num]
	m.mu.RUnlock()

	return NodeRef{
		Num:       num,
		ID:        node.ID,
		LongName:  node.LongName,
		ShortName: node.ShortName,
	}
}

var ErrClosed = errors.New("mesh closed")
