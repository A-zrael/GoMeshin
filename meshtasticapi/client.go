package meshtasticapi

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	meshtastic "gitplac.si/gomeshtastic/protobuf/v2"
	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

const (
	start1         = 0x94
	start2         = 0xc3
	maxPayloadSize = 512

	BroadcastNode = uint32(0xffffffff)
)

type Config struct {
	Port string
	Baud int
}

type Client struct {
	port serial.Port

	events chan Event
	errors chan error
	closed chan struct{}

	writeMu  sync.Mutex
	stateMu  sync.RWMutex
	closeMu  sync.Once
	packetID uint32
	hopLimit uint32
	channels map[int32]Channel
	nodes    map[uint32]Node
	myNode   *MyNode
}

type Event struct {
	Type EventType

	Packet     *Packet
	Node       *Node
	MyNode     *MyNode
	Channel    *Channel
	TraceRoute *TraceRoute
	Log        string
	Raw        *meshtastic.FromRadio
}

type EventType string

const (
	EventText           EventType = "text"
	EventPacket         EventType = "packet"
	EventEncrypted      EventType = "encrypted"
	EventNode           EventType = "node"
	EventMyNode         EventType = "my_node"
	EventChannel        EventType = "channel"
	EventTraceRoute     EventType = "traceroute"
	EventLog            EventType = "log"
	EventConfigComplete EventType = "config_complete"
	EventRebooted       EventType = "rebooted"
	EventRaw            EventType = "raw"
)

type Packet struct {
	ID        uint32
	From      uint32
	To        uint32
	Channel   uint32
	Port      meshtastic.PortNum
	Payload   []byte
	Text      string
	RSSI      int32
	SNR       float32
	RxTime    time.Time
	WantAck   bool
	Encrypted bool
}

type Node struct {
	Num       uint32
	ID        string
	LongName  string
	ShortName string
}

type MyNode struct {
	Num         uint32
	PIOEnv      string
	RebootCount uint32
}

type TraceRoute struct {
	RequestID  uint32
	From       uint32
	To         uint32
	Route      []uint32
	SNRTowards []int32
	RouteBack  []uint32
	SNRBack    []int32
}

type Channel struct {
	Index           int32
	Name            string
	Role            meshtastic.Channel_Role
	ID              uint32
	PSKBytes        int
	UplinkEnabled   bool
	DownlinkEnabled bool
}

type SendOptions struct {
	To       uint32
	Channel  uint32
	Port     meshtastic.PortNum
	Payload  []byte
	WantAck  bool
	WantResp bool
	HopLimit uint32
	ReplyID  uint32
	PKI      bool
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Port == "" {
		return nil, errors.New("serial port is required")
	}
	if cfg.Baud == 0 {
		cfg.Baud = 115200
	}

	port, err := serial.Open(cfg.Port, &serial.Mode{
		BaudRate: cfg.Baud,
		InitialStatusBits: &serial.ModemOutputBits{
			DTR: false,
			RTS: false,
		},
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		port:     port,
		events:   make(chan Event, 64),
		errors:   make(chan error, 4),
		closed:   make(chan struct{}),
		packetID: uint32(rand.New(rand.NewSource(time.Now().UnixNano())).Int31()),
		hopLimit: 3,
		channels: make(map[int32]Channel),
		nodes:    make(map[uint32]Node),
	}

	if err := c.wakeRadio(); err != nil {
		_ = port.Close()
		return nil, err
	}

	go c.readLoop()

	configID := uint32(time.Now().UnixNano())
	if err := c.sendWantConfig(configID); err != nil {
		_ = c.Close()
		return nil, err
	}

	if err := c.waitForConfig(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}

	return c, nil
}

func (c *Client) Events() <-chan Event {
	return c.events
}

func (c *Client) Errors() <-chan error {
	return c.errors
}

func (c *Client) Channels() []Channel {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	channels := make([]Channel, 0, len(c.channels))
	for _, channel := range c.channels {
		channels = append(channels, channel)
	}

	for i := 0; i < len(channels); i++ {
		for j := i + 1; j < len(channels); j++ {
			if channels[j].Index < channels[i].Index {
				channels[i], channels[j] = channels[j], channels[i]
			}
		}
	}

	return channels
}

func (c *Client) Nodes() []Node {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	nodes := make([]Node, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (c *Client) MyNode() *MyNode {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	if c.myNode == nil {
		return nil
	}

	node := *c.myNode
	return &node
}

func (c *Client) Close() error {
	var err error
	c.closeMu.Do(func() {
		close(c.closed)
		err = c.port.Close()
	})
	return err
}

func (c *Client) SendText(text string, opts SendOptions) (uint32, error) {
	opts.Port = meshtastic.PortNum_TEXT_MESSAGE_APP
	opts.Payload = []byte(text)
	return c.SendData(opts)
}

func (c *Client) SendData(opts SendOptions) (uint32, error) {
	if opts.To == 0 {
		opts.To = BroadcastNode
	}
	if opts.Port == meshtastic.PortNum_UNKNOWN_APP {
		return 0, errors.New("port is required")
	}

	id := c.nextPacketID()
	data := &meshtastic.Data{
		Portnum:      opts.Port,
		Payload:      opts.Payload,
		WantResponse: opts.WantResp,
	}
	if opts.ReplyID != 0 {
		data.ReplyId = opts.ReplyID
	}

	packet := &meshtastic.MeshPacket{
		To:           opts.To,
		Channel:      opts.Channel,
		Id:           id,
		WantAck:      opts.WantAck,
		PkiEncrypted: opts.PKI,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: data,
		},
	}
	if opts.HopLimit != 0 {
		packet.HopLimit = opts.HopLimit
	} else {
		packet.HopLimit = c.hopLimit
	}

	msg := &meshtastic.ToRadio{
		PayloadVariant: &meshtastic.ToRadio_Packet{
			Packet: packet,
		},
	}

	return id, c.writeFrame(msg)
}

func (c *Client) wakeRadio() error {
	_ = c.port.SetDTR(false)
	_ = c.port.SetRTS(false)
	_ = c.port.ResetInputBuffer()
	_ = c.port.ResetOutputBuffer()

	wake := make([]byte, 32)
	for i := range wake {
		wake[i] = start2
	}
	if _, err := c.port.Write(wake); err != nil {
		return err
	}
	if err := c.port.Drain(); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)
	return nil
}

func (c *Client) sendWantConfig(configID uint32) error {
	return c.writeFrame(&meshtastic.ToRadio{
		PayloadVariant: &meshtastic.ToRadio_WantConfigId{
			WantConfigId: configID,
		},
	})
}

func (c *Client) waitForConfig(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for config: %w", ctx.Err())
		case event := <-c.events:
			if event.Type == EventConfigComplete {
				return nil
			}
		case err := <-c.errors:
			return err
		case <-c.closed:
			return errors.New("client closed before config completed")
		}
	}
}

func (c *Client) readLoop() {
	defer close(c.events)
	defer close(c.errors)

	for {
		payload, err := readFrame(c.port)
		if err != nil {
			select {
			case <-c.closed:
			case c.errors <- err:
			}
			return
		}

		var fromRadio meshtastic.FromRadio
		if err := proto.Unmarshal(payload, &fromRadio); err != nil {
			c.publishError(fmt.Errorf("unmarshal FromRadio: %w", err))
			continue
		}

		for _, event := range decodeFromRadio(&fromRadio) {
			c.updateState(event)
			select {
			case <-c.closed:
				return
			case c.events <- event:
			}
		}
	}
}

func (c *Client) updateState(event Event) {
	switch event.Type {
	case EventChannel:
		if event.Channel == nil {
			return
		}
		c.stateMu.Lock()
		c.channels[event.Channel.Index] = *event.Channel
		c.stateMu.Unlock()
	case EventMyNode:
		if event.MyNode == nil {
			return
		}
		node := *event.MyNode
		c.stateMu.Lock()
		c.myNode = &node
		c.stateMu.Unlock()
	case EventNode:
		if event.Node == nil {
			return
		}
		node := *event.Node
		c.stateMu.Lock()
		c.nodes[node.Num] = node
		c.stateMu.Unlock()
	}
}

func (c *Client) publishError(err error) {
	select {
	case <-c.closed:
	case c.errors <- err:
	default:
	}
}

func (c *Client) writeFrame(msg proto.Message) error {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	if len(payload) > maxPayloadSize {
		return fmt.Errorf("payload too large: %d bytes", len(payload))
	}

	frame := make([]byte, 4+len(payload))
	frame[0] = start1
	frame[1] = start2
	binary.BigEndian.PutUint16(frame[2:4], uint16(len(payload)))
	copy(frame[4:], payload)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	for len(frame) > 0 {
		n, err := c.port.Write(frame)
		if err != nil {
			return err
		}
		frame = frame[n:]
	}

	return c.port.Drain()
}

func (c *Client) nextPacketID() uint32 {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	next := (c.packetID + 1) & 0x3ff
	randomPart := uint32(rand.Int31n(0x400000)) << 10
	c.packetID = next | randomPart
	return c.packetID
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)

	for {
		if _, err := io.ReadFull(r, header[:1]); err != nil {
			return nil, err
		}
		if header[0] != start1 {
			continue
		}

		if _, err := io.ReadFull(r, header[1:2]); err != nil {
			return nil, err
		}
		if header[1] != start2 {
			continue
		}

		if _, err := io.ReadFull(r, header[2:]); err != nil {
			return nil, err
		}

		length := int(binary.BigEndian.Uint16(header[2:]))
		if length <= 0 || length > maxPayloadSize {
			continue
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}

		return payload, nil
	}
}

func decodeFromRadio(msg *meshtastic.FromRadio) []Event {
	switch payload := msg.GetPayloadVariant().(type) {
	case *meshtastic.FromRadio_Packet:
		return []Event{decodeMeshPacket(payload.Packet)}
	case *meshtastic.FromRadio_MyInfo:
		info := payload.MyInfo
		return []Event{{
			Type: EventMyNode,
			MyNode: &MyNode{
				Num:         info.GetMyNodeNum(),
				PIOEnv:      info.GetPioEnv(),
				RebootCount: info.GetRebootCount(),
			},
			Raw: msg,
		}}
	case *meshtastic.FromRadio_NodeInfo:
		node := payload.NodeInfo
		user := node.GetUser()
		return []Event{{
			Type: EventNode,
			Node: &Node{
				Num:       node.GetNum(),
				ID:        user.GetId(),
				LongName:  user.GetLongName(),
				ShortName: user.GetShortName(),
			},
			Raw: msg,
		}}
	case *meshtastic.FromRadio_Channel:
		channel := decodeChannel(payload.Channel)
		return []Event{{
			Type:    EventChannel,
			Channel: &channel,
			Raw:     msg,
		}}
	case *meshtastic.FromRadio_LogRecord:
		return []Event{{
			Type: EventLog,
			Log:  payload.LogRecord.GetMessage(),
			Raw:  msg,
		}}
	case *meshtastic.FromRadio_ConfigCompleteId:
		return []Event{{
			Type: EventConfigComplete,
			Raw:  msg,
		}}
	case *meshtastic.FromRadio_Rebooted:
		return []Event{{
			Type: EventRebooted,
			Raw:  msg,
		}}
	default:
		return []Event{{
			Type: EventRaw,
			Raw:  msg,
		}}
	}
}

func decodeChannel(channel *meshtastic.Channel) Channel {
	if channel == nil {
		return Channel{}
	}

	decoded := Channel{
		Index: channel.GetIndex(),
		Role:  channel.GetRole(),
	}

	settings := channel.GetSettings()
	if settings != nil {
		decoded.Name = settings.GetName()
		decoded.ID = settings.GetId()
		decoded.PSKBytes = len(settings.GetPsk())
		decoded.UplinkEnabled = settings.GetUplinkEnabled()
		decoded.DownlinkEnabled = settings.GetDownlinkEnabled()
	}

	if decoded.Name == "" && decoded.Role == meshtastic.Channel_PRIMARY {
		decoded.Name = "Primary"
	}

	return decoded
}

func decodeMeshPacket(packet *meshtastic.MeshPacket) Event {
	if packet == nil {
		return Event{Type: EventPacket}
	}

	event := Event{
		Type: EventPacket,
		Packet: &Packet{
			ID:      packet.GetId(),
			From:    packet.GetFrom(),
			To:      packet.GetTo(),
			Channel: packet.GetChannel(),
			RSSI:    packet.GetRxRssi(),
			SNR:     packet.GetRxSnr(),
			WantAck: packet.GetWantAck(),
		},
	}
	if packet.GetRxTime() != 0 {
		event.Packet.RxTime = time.Unix(int64(packet.GetRxTime()), 0)
	}

	decoded := packet.GetDecoded()
	if decoded == nil {
		event.Type = EventEncrypted
		event.Packet.Encrypted = true
		event.Packet.Payload = packet.GetEncrypted()
		return event
	}

	event.Packet.Port = decoded.GetPortnum()
	event.Packet.Payload = decoded.GetPayload()

	switch decoded.GetPortnum() {
	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		event.Type = EventText
		event.Packet.Text = string(decoded.GetPayload())
	case meshtastic.PortNum_TEXT_MESSAGE_COMPRESSED_APP:
		event.Type = EventText
		event.Packet.Text = string(decoded.GetPayload())
	case meshtastic.PortNum_TRACEROUTE_APP:
		var route meshtastic.RouteDiscovery
		if err := proto.Unmarshal(decoded.GetPayload(), &route); err == nil {
			event.Type = EventTraceRoute
			event.TraceRoute = &TraceRoute{
				RequestID:  decoded.GetRequestId(),
				From:       packet.GetFrom(),
				To:         packet.GetTo(),
				Route:      append([]uint32(nil), route.GetRoute()...),
				SNRTowards: append([]int32(nil), route.GetSnrTowards()...),
				RouteBack:  append([]uint32(nil), route.GetRouteBack()...),
				SNRBack:    append([]int32(nil), route.GetSnrBack()...),
			}
		}
	default:
		event.Type = EventPacket
	}

	return event
}
