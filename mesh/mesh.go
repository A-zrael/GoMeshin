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

	mu           sync.RWMutex
	myNode       uint32
	nodes        map[uint32]Node
	positions    map[uint32]Position
	environments map[uint32]EnvironmentTelemetry
	channels     map[int32]Channel

	subsMu                 sync.RWMutex
	subscribers            map[chan Message]struct{}
	positionSubscribers    map[chan Position]struct{}
	environmentSubscribers map[chan EnvironmentTelemetry]struct{}

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

type Position struct {
	Node          NodeRef
	Latitude      float64
	Longitude     float64
	Altitude      *int32
	AltitudeHAE   *int32
	GroundSpeed   *uint32
	GroundTrack   *uint32
	SatsInView    uint32
	PrecisionBits uint32
	Timestamp     time.Time
	ReceivedAt    time.Time
}

type EnvironmentTelemetry struct {
	Node               NodeRef
	Temperature        *float32
	RelativeHumidity   *float32
	BarometricPressure *float32
	GasResistance      *float32
	Voltage            *float32
	Current            *float32
	IAQ                *uint32
	Distance           *float32
	Lux                *float32
	WhiteLux           *float32
	IRLux              *float32
	UVLux              *float32
	WindDirection      *uint32
	WindSpeed          *float32
	WindGust           *float32
	WindLull           *float32
	Weight             *float32
	Timestamp          time.Time
	ReceivedAt         time.Time
}

type Node struct {
	Num         uint32
	ID          string
	LongName    string
	ShortName   string
	LastSeen    time.Time
	Position    *Position
	Environment *EnvironmentTelemetry
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
		radio:                  radio,
		store:                  cfg.Store,
		ctx:                    runCtx,
		cancel:                 cancel,
		done:                   make(chan struct{}),
		nodes:                  make(map[uint32]Node),
		positions:              make(map[uint32]Position),
		environments:           make(map[uint32]EnvironmentTelemetry),
		channels:               make(map[int32]Channel),
		subscribers:            make(map[chan Message]struct{}),
		positionSubscribers:    make(map[chan Position]struct{}),
		environmentSubscribers: make(map[chan EnvironmentTelemetry]struct{}),
		traces:                 make(map[uint32]chan TraceRoute),
	}

	for _, channel := range radio.Channels() {
		m.upsertChannel(convertChannel(channel))
	}
	for _, node := range radio.Nodes() {
		meshNode := Node{
			Num:       node.Num,
			ID:        node.ID,
			LongName:  node.LongName,
			ShortName: node.ShortName,
			LastSeen:  time.Now(),
		}
		m.upsertNode(meshNode)
		if node.Position != nil {
			m.upsertPosition(convertPosition(*node.Position, m.nodeRef))
		}
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

func (m *Mesh) SubscribePositions(buffer int) (<-chan Position, func()) {
	if buffer < 0 {
		buffer = 0
	}

	ch := make(chan Position, buffer)
	m.subsMu.Lock()
	m.positionSubscribers[ch] = struct{}{}
	m.subsMu.Unlock()

	cancel := func() {
		m.subsMu.Lock()
		if _, ok := m.positionSubscribers[ch]; ok {
			delete(m.positionSubscribers, ch)
			close(ch)
		}
		m.subsMu.Unlock()
	}

	return ch, cancel
}

func (m *Mesh) SubscribeEnvironment(buffer int) (<-chan EnvironmentTelemetry, func()) {
	if buffer < 0 {
		buffer = 0
	}

	ch := make(chan EnvironmentTelemetry, buffer)
	m.subsMu.Lock()
	m.environmentSubscribers[ch] = struct{}{}
	m.subsMu.Unlock()

	cancel := func() {
		m.subsMu.Lock()
		if _, ok := m.environmentSubscribers[ch]; ok {
			delete(m.environmentSubscribers, ch)
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

func (m *Mesh) Positions(ctx context.Context) ([]Position, error) {
	return m.store.Positions(ctx)
}

func (m *Mesh) EnvironmentTelemetries(ctx context.Context) ([]EnvironmentTelemetry, error) {
	return m.store.EnvironmentTelemetries(ctx)
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
		if event.Node.Position != nil {
			m.upsertPosition(convertPosition(*event.Node.Position, m.nodeRef))
		}
	case meshtasticapi.EventChannel:
		if event.Channel == nil {
			return
		}
		m.upsertChannel(convertChannel(*event.Channel))
	case meshtasticapi.EventPosition:
		if event.Position == nil {
			return
		}
		m.upsertPosition(convertPosition(*event.Position, m.nodeRef))
	case meshtasticapi.EventEnvironment:
		if event.Environment == nil {
			return
		}
		m.upsertEnvironment(convertEnvironmentTelemetry(*event.Environment, m.nodeRef))
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
	if position, ok := m.positions[node.Num]; ok && node.Position == nil {
		node.Position = &position
	}
	if environment, ok := m.environments[node.Num]; ok && node.Environment == nil {
		node.Environment = &environment
	}
	m.nodes[node.Num] = node
	m.mu.Unlock()

	_ = m.store.SaveNode(m.ctx, node)
}

func (m *Mesh) upsertPosition(position Position) {
	if position.ReceivedAt.IsZero() {
		position.ReceivedAt = time.Now()
	}

	m.mu.Lock()
	m.positions[position.Node.Num] = position
	if node, ok := m.nodes[position.Node.Num]; ok {
		node.Position = &position
		m.nodes[position.Node.Num] = node
		_ = m.store.SaveNode(m.ctx, node)
	}
	m.mu.Unlock()

	_ = m.store.SavePosition(m.ctx, position)
	m.publishPosition(position)
}

func (m *Mesh) upsertEnvironment(environment EnvironmentTelemetry) {
	if environment.ReceivedAt.IsZero() {
		environment.ReceivedAt = time.Now()
	}

	m.mu.Lock()
	m.environments[environment.Node.Num] = environment
	if node, ok := m.nodes[environment.Node.Num]; ok {
		node.Environment = &environment
		m.nodes[environment.Node.Num] = node
		_ = m.store.SaveNode(m.ctx, node)
	}
	m.mu.Unlock()

	_ = m.store.SaveEnvironmentTelemetry(m.ctx, environment)
	m.publishEnvironment(environment)
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

func (m *Mesh) publishPosition(position Position) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	for ch := range m.positionSubscribers {
		select {
		case ch <- position:
		default:
		}
	}
}

func (m *Mesh) publishEnvironment(environment EnvironmentTelemetry) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	for ch := range m.environmentSubscribers {
		select {
		case ch <- environment:
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
	for ch := range m.positionSubscribers {
		close(ch)
		delete(m.positionSubscribers, ch)
	}
	for ch := range m.environmentSubscribers {
		close(ch)
		delete(m.environmentSubscribers, ch)
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

func convertPosition(position meshtasticapi.Position, lookup func(uint32) NodeRef) Position {
	return Position{
		Node:          lookup(position.NodeNum),
		Latitude:      position.Latitude,
		Longitude:     position.Longitude,
		Altitude:      position.Altitude,
		AltitudeHAE:   position.AltitudeHAE,
		GroundSpeed:   position.GroundSpeed,
		GroundTrack:   position.GroundTrack,
		SatsInView:    position.SatsInView,
		PrecisionBits: position.PrecisionBits,
		Timestamp:     position.Timestamp,
		ReceivedAt:    position.ReceivedAt,
	}
}

func convertEnvironmentTelemetry(environment meshtasticapi.EnvironmentTelemetry, lookup func(uint32) NodeRef) EnvironmentTelemetry {
	return EnvironmentTelemetry{
		Node:               lookup(environment.NodeNum),
		Temperature:        environment.Temperature,
		RelativeHumidity:   environment.RelativeHumidity,
		BarometricPressure: environment.BarometricPressure,
		GasResistance:      environment.GasResistance,
		Voltage:            environment.Voltage,
		Current:            environment.Current,
		IAQ:                environment.IAQ,
		Distance:           environment.Distance,
		Lux:                environment.Lux,
		WhiteLux:           environment.WhiteLux,
		IRLux:              environment.IRLux,
		UVLux:              environment.UVLux,
		WindDirection:      environment.WindDirection,
		WindSpeed:          environment.WindSpeed,
		WindGust:           environment.WindGust,
		WindLull:           environment.WindLull,
		Weight:             environment.Weight,
		Timestamp:          environment.Timestamp,
		ReceivedAt:         environment.ReceivedAt,
	}
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
