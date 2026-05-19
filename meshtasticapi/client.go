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
	env      map[uint32]EnvironmentTelemetry
	device   map[uint32]DeviceTelemetry
	power    map[uint32]PowerTelemetry
	air      map[uint32]AirQualityTelemetry
	local    map[uint32]LocalStatsTelemetry
	health   map[uint32]HealthTelemetry
	myNode   *MyNode
}

type Event struct {
	Type EventType

	Packet      *Packet
	Node        *Node
	Position    *Position
	Environment *EnvironmentTelemetry
	Device      *DeviceTelemetry
	Power       *PowerTelemetry
	AirQuality  *AirQualityTelemetry
	LocalStats  *LocalStatsTelemetry
	Health      *HealthTelemetry
	MyNode      *MyNode
	Channel     *Channel
	TraceRoute  *TraceRoute
	Log         string
	Raw         *meshtastic.FromRadio
}

type EventType string

const (
	EventText           EventType = "text"
	EventPacket         EventType = "packet"
	EventEncrypted      EventType = "encrypted"
	EventNode           EventType = "node"
	EventPosition       EventType = "position"
	EventEnvironment    EventType = "environment"
	EventDevice         EventType = "device_telemetry"
	EventPower          EventType = "power_telemetry"
	EventAirQuality     EventType = "air_quality_telemetry"
	EventLocalStats     EventType = "local_stats_telemetry"
	EventHealth         EventType = "health_telemetry"
	EventMyNode         EventType = "my_node"
	EventChannel        EventType = "channel"
	EventTraceRoute     EventType = "traceroute"
	EventLog            EventType = "log"
	EventConfigComplete EventType = "config_complete"
	EventRebooted       EventType = "rebooted"
	EventRaw            EventType = "raw"
)

type Packet struct {
	ID          uint32
	RequestID   uint32
	ReplyID     uint32
	From        uint32
	To          uint32
	Channel     uint32
	Port        meshtastic.PortNum
	Payload     []byte
	Text        string
	Position    *Position
	Environment *EnvironmentTelemetry
	Device      *DeviceTelemetry
	Power       *PowerTelemetry
	AirQuality  *AirQualityTelemetry
	LocalStats  *LocalStatsTelemetry
	Health      *HealthTelemetry
	RSSI        int32
	SNR         float32
	RxTime      time.Time
	WantAck     bool
	Encrypted   bool
}

type Node struct {
	Num       uint32
	ID        string
	LongName  string
	ShortName string
	Position  *Position
}

type Position struct {
	NodeNum       uint32
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
	NodeNum            uint32
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

type DeviceTelemetry struct {
	NodeNum            uint32
	BatteryLevel       *uint32
	Voltage            *float32
	ChannelUtilization *float32
	AirUtilTx          *float32
	UptimeSeconds      *uint32
	Timestamp          time.Time
	ReceivedAt         time.Time
}

type PowerTelemetry struct {
	NodeNum    uint32
	Ch1Voltage *float32
	Ch1Current *float32
	Ch2Voltage *float32
	Ch2Current *float32
	Ch3Voltage *float32
	Ch3Current *float32
	Timestamp  time.Time
	ReceivedAt time.Time
}

type AirQualityTelemetry struct {
	NodeNum            uint32
	Pm10Standard       *uint32
	Pm25Standard       *uint32
	Pm100Standard      *uint32
	Pm10Environmental  *uint32
	Pm25Environmental  *uint32
	Pm100Environmental *uint32
	Particles03um      *uint32
	Particles05um      *uint32
	Particles10um      *uint32
	Particles25um      *uint32
	Particles50um      *uint32
	Particles100um     *uint32
	CO2                *uint32
	Timestamp          time.Time
	ReceivedAt         time.Time
}

type LocalStatsTelemetry struct {
	NodeNum            uint32
	UptimeSeconds      uint32
	ChannelUtilization float32
	AirUtilTx          float32
	NumPacketsTx       uint32
	NumPacketsRx       uint32
	NumPacketsRxBad    uint32
	NumOnlineNodes     uint32
	NumTotalNodes      uint32
	NumRxDupe          uint32
	NumTxRelay         uint32
	NumTxRelayCanceled uint32
	Timestamp          time.Time
	ReceivedAt         time.Time
}

type HealthTelemetry struct {
	NodeNum     uint32
	HeartBPM    *uint32
	SpO2        *uint32
	Temperature *float32
	Timestamp   time.Time
	ReceivedAt  time.Time
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
	RxRSSI     *int32
	RxSNR      *float32
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
		env:      make(map[uint32]EnvironmentTelemetry),
		device:   make(map[uint32]DeviceTelemetry),
		power:    make(map[uint32]PowerTelemetry),
		air:      make(map[uint32]AirQualityTelemetry),
		local:    make(map[uint32]LocalStatsTelemetry),
		health:   make(map[uint32]HealthTelemetry),
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

func (c *Client) EnvironmentTelemetries() []EnvironmentTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	environments := make([]EnvironmentTelemetry, 0, len(c.env))
	for _, environment := range c.env {
		environments = append(environments, environment)
	}
	return environments
}

func (c *Client) DeviceTelemetries() []DeviceTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	out := make([]DeviceTelemetry, 0, len(c.device))
	for _, sample := range c.device {
		out = append(out, sample)
	}
	return out
}

func (c *Client) PowerTelemetries() []PowerTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	out := make([]PowerTelemetry, 0, len(c.power))
	for _, sample := range c.power {
		out = append(out, sample)
	}
	return out
}

func (c *Client) AirQualityTelemetries() []AirQualityTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	out := make([]AirQualityTelemetry, 0, len(c.air))
	for _, sample := range c.air {
		out = append(out, sample)
	}
	return out
}

func (c *Client) LocalStatsTelemetries() []LocalStatsTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	out := make([]LocalStatsTelemetry, 0, len(c.local))
	for _, sample := range c.local {
		out = append(out, sample)
	}
	return out
}

func (c *Client) HealthTelemetries() []HealthTelemetry {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	out := make([]HealthTelemetry, 0, len(c.health))
	for _, sample := range c.health {
		out = append(out, sample)
	}
	return out
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
		case event, ok := <-c.events:
			if !ok {
				return errors.New("event stream closed before config completed")
			}
			if event.Type == EventConfigComplete {
				return nil
			}
		case err, ok := <-c.errors:
			if !ok {
				return errors.New("error stream closed before config completed")
			}
			if err == nil {
				return errors.New("radio closed before config completed")
			}
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
	case EventPosition:
		if event.Position == nil {
			return
		}
		position := *event.Position
		c.stateMu.Lock()
		node := c.nodes[position.NodeNum]
		node.Num = position.NodeNum
		node.Position = &position
		c.nodes[position.NodeNum] = node
		c.stateMu.Unlock()
	case EventEnvironment:
		if event.Environment == nil {
			return
		}
		environment := *event.Environment
		c.stateMu.Lock()
		c.env[environment.NodeNum] = environment
		c.stateMu.Unlock()
	case EventDevice:
		if event.Device == nil {
			return
		}
		sample := *event.Device
		c.stateMu.Lock()
		c.device[sample.NodeNum] = sample
		c.stateMu.Unlock()
	case EventPower:
		if event.Power == nil {
			return
		}
		sample := *event.Power
		c.stateMu.Lock()
		c.power[sample.NodeNum] = sample
		c.stateMu.Unlock()
	case EventAirQuality:
		if event.AirQuality == nil {
			return
		}
		sample := *event.AirQuality
		c.stateMu.Lock()
		c.air[sample.NodeNum] = sample
		c.stateMu.Unlock()
	case EventLocalStats:
		if event.LocalStats == nil {
			return
		}
		sample := *event.LocalStats
		c.stateMu.Lock()
		c.local[sample.NodeNum] = sample
		c.stateMu.Unlock()
	case EventHealth:
		if event.Health == nil {
			return
		}
		sample := *event.Health
		c.stateMu.Lock()
		c.health[sample.NodeNum] = sample
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
		decodedNode := Node{
			Num:       node.GetNum(),
			ID:        user.GetId(),
			LongName:  user.GetLongName(),
			ShortName: user.GetShortName(),
		}
		if position := decodePosition(node.GetPosition(), node.GetNum(), time.Now()); position != nil {
			decodedNode.Position = position
		}
		return []Event{{
			Type: EventNode,
			Node: &decodedNode,
			Raw:  msg,
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

func decodePosition(position *meshtastic.Position, nodeNum uint32, receivedAt time.Time) *Position {
	if position == nil || position.LatitudeI == nil || position.LongitudeI == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	timestamp := receivedAt
	switch {
	case position.GetTimestamp() != 0:
		timestamp = time.Unix(int64(position.GetTimestamp()), 0)
	case position.GetTime() != 0:
		timestamp = time.Unix(int64(position.GetTime()), 0)
	}

	decoded := &Position{
		NodeNum:       nodeNum,
		Latitude:      float64(position.GetLatitudeI()) * 1e-7,
		Longitude:     float64(position.GetLongitudeI()) * 1e-7,
		SatsInView:    position.GetSatsInView(),
		PrecisionBits: position.GetPrecisionBits(),
		Timestamp:     timestamp,
		ReceivedAt:    receivedAt,
	}
	if position.Altitude != nil {
		value := position.GetAltitude()
		decoded.Altitude = &value
	}
	if position.AltitudeHae != nil {
		value := position.GetAltitudeHae()
		decoded.AltitudeHAE = &value
	}
	if position.GroundSpeed != nil {
		value := position.GetGroundSpeed()
		decoded.GroundSpeed = &value
	}
	if position.GroundTrack != nil {
		value := position.GetGroundTrack()
		decoded.GroundTrack = &value
	}
	return decoded
}

func decodeEnvironmentTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *EnvironmentTelemetry {
	metrics := telemetry.GetEnvironmentMetrics()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}

	return &EnvironmentTelemetry{
		NodeNum:            nodeNum,
		Temperature:        float32Ptr(metrics.Temperature),
		RelativeHumidity:   float32Ptr(metrics.RelativeHumidity),
		BarometricPressure: float32Ptr(metrics.BarometricPressure),
		GasResistance:      float32Ptr(metrics.GasResistance),
		Voltage:            float32Ptr(metrics.Voltage),
		Current:            float32Ptr(metrics.Current),
		IAQ:                uint32Ptr(metrics.Iaq),
		Distance:           float32Ptr(metrics.Distance),
		Lux:                float32Ptr(metrics.Lux),
		WhiteLux:           float32Ptr(metrics.WhiteLux),
		IRLux:              float32Ptr(metrics.IrLux),
		UVLux:              float32Ptr(metrics.UvLux),
		WindDirection:      uint32Ptr(metrics.WindDirection),
		WindSpeed:          float32Ptr(metrics.WindSpeed),
		WindGust:           float32Ptr(metrics.WindGust),
		WindLull:           float32Ptr(metrics.WindLull),
		Weight:             float32Ptr(metrics.Weight),
		Timestamp:          timestamp,
		ReceivedAt:         receivedAt,
	}
}

func decodeDeviceTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *DeviceTelemetry {
	metrics := telemetry.GetDeviceMetrics()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}
	return &DeviceTelemetry{
		NodeNum:            nodeNum,
		BatteryLevel:       uint32Ptr(metrics.BatteryLevel),
		Voltage:            float32Ptr(metrics.Voltage),
		ChannelUtilization: float32Ptr(metrics.ChannelUtilization),
		AirUtilTx:          float32Ptr(metrics.AirUtilTx),
		UptimeSeconds:      uint32Ptr(metrics.UptimeSeconds),
		Timestamp:          timestamp,
		ReceivedAt:         receivedAt,
	}
}

func decodePowerTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *PowerTelemetry {
	metrics := telemetry.GetPowerMetrics()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}
	return &PowerTelemetry{
		NodeNum:    nodeNum,
		Ch1Voltage: float32Ptr(metrics.Ch1Voltage),
		Ch1Current: float32Ptr(metrics.Ch1Current),
		Ch2Voltage: float32Ptr(metrics.Ch2Voltage),
		Ch2Current: float32Ptr(metrics.Ch2Current),
		Ch3Voltage: float32Ptr(metrics.Ch3Voltage),
		Ch3Current: float32Ptr(metrics.Ch3Current),
		Timestamp:  timestamp,
		ReceivedAt: receivedAt,
	}
}

func decodeAirQualityTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *AirQualityTelemetry {
	metrics := telemetry.GetAirQualityMetrics()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}
	return &AirQualityTelemetry{
		NodeNum:            nodeNum,
		Pm10Standard:       uint32Ptr(metrics.Pm10Standard),
		Pm25Standard:       uint32Ptr(metrics.Pm25Standard),
		Pm100Standard:      uint32Ptr(metrics.Pm100Standard),
		Pm10Environmental:  uint32Ptr(metrics.Pm10Environmental),
		Pm25Environmental:  uint32Ptr(metrics.Pm25Environmental),
		Pm100Environmental: uint32Ptr(metrics.Pm100Environmental),
		Particles03um:      uint32Ptr(metrics.Particles_03Um),
		Particles05um:      uint32Ptr(metrics.Particles_05Um),
		Particles10um:      uint32Ptr(metrics.Particles_10Um),
		Particles25um:      uint32Ptr(metrics.Particles_25Um),
		Particles50um:      uint32Ptr(metrics.Particles_50Um),
		Particles100um:     uint32Ptr(metrics.Particles_100Um),
		CO2:                uint32Ptr(metrics.Co2),
		Timestamp:          timestamp,
		ReceivedAt:         receivedAt,
	}
}

func decodeLocalStatsTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *LocalStatsTelemetry {
	metrics := telemetry.GetLocalStats()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}
	return &LocalStatsTelemetry{
		NodeNum:            nodeNum,
		UptimeSeconds:      metrics.GetUptimeSeconds(),
		ChannelUtilization: metrics.GetChannelUtilization(),
		AirUtilTx:          metrics.GetAirUtilTx(),
		NumPacketsTx:       metrics.GetNumPacketsTx(),
		NumPacketsRx:       metrics.GetNumPacketsRx(),
		NumPacketsRxBad:    metrics.GetNumPacketsRxBad(),
		NumOnlineNodes:     metrics.GetNumOnlineNodes(),
		NumTotalNodes:      metrics.GetNumTotalNodes(),
		NumRxDupe:          metrics.GetNumRxDupe(),
		NumTxRelay:         metrics.GetNumTxRelay(),
		NumTxRelayCanceled: metrics.GetNumTxRelayCanceled(),
		Timestamp:          timestamp,
		ReceivedAt:         receivedAt,
	}
}

func decodeHealthTelemetry(telemetry *meshtastic.Telemetry, nodeNum uint32, receivedAt time.Time) *HealthTelemetry {
	metrics := telemetry.GetHealthMetrics()
	if metrics == nil {
		return nil
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := receivedAt
	if telemetry.GetTime() != 0 {
		timestamp = time.Unix(int64(telemetry.GetTime()), 0)
	}
	return &HealthTelemetry{
		NodeNum:     nodeNum,
		HeartBPM:    uint32Ptr(metrics.HeartBpm),
		SpO2:        uint32Ptr(metrics.SpO2),
		Temperature: float32Ptr(metrics.Temperature),
		Timestamp:   timestamp,
		ReceivedAt:  receivedAt,
	}
}

func float32Ptr(value *float32) *float32 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func uint32Ptr(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
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
	event.Packet.RequestID = decoded.GetRequestId()
	event.Packet.ReplyID = decoded.GetReplyId()
	event.Packet.Payload = decoded.GetPayload()

	switch decoded.GetPortnum() {
	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		event.Type = EventText
		event.Packet.Text = string(decoded.GetPayload())
	case meshtastic.PortNum_TEXT_MESSAGE_COMPRESSED_APP:
		event.Type = EventText
		event.Packet.Text = string(decoded.GetPayload())
	case meshtastic.PortNum_POSITION_APP:
		var position meshtastic.Position
		if err := proto.Unmarshal(decoded.GetPayload(), &position); err == nil {
			if decodedPosition := decodePosition(&position, packet.GetFrom(), event.Packet.RxTime); decodedPosition != nil {
				event.Type = EventPosition
				event.Packet.Position = decodedPosition
				event.Position = decodedPosition
			}
		}
	case meshtastic.PortNum_TELEMETRY_APP:
		var telemetry meshtastic.Telemetry
		if err := proto.Unmarshal(decoded.GetPayload(), &telemetry); err == nil {
			if environment := decodeEnvironmentTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); environment != nil {
				event.Type = EventEnvironment
				event.Packet.Environment = environment
				event.Environment = environment
			} else if device := decodeDeviceTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); device != nil {
				event.Type = EventDevice
				event.Packet.Device = device
				event.Device = device
			} else if power := decodePowerTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); power != nil {
				event.Type = EventPower
				event.Packet.Power = power
				event.Power = power
			} else if air := decodeAirQualityTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); air != nil {
				event.Type = EventAirQuality
				event.Packet.AirQuality = air
				event.AirQuality = air
			} else if local := decodeLocalStatsTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); local != nil {
				event.Type = EventLocalStats
				event.Packet.LocalStats = local
				event.LocalStats = local
			} else if health := decodeHealthTelemetry(&telemetry, packet.GetFrom(), event.Packet.RxTime); health != nil {
				event.Type = EventHealth
				event.Packet.Health = health
				event.Health = health
			}
		}
	case meshtastic.PortNum_TRACEROUTE_APP:
		var route meshtastic.RouteDiscovery
		if err := proto.Unmarshal(decoded.GetPayload(), &route); err == nil {
			event.Type = EventTraceRoute
			event.TraceRoute = decodeTraceRoute(decoded, packet, &route)
		}
	case meshtastic.PortNum_ROUTING_APP:
		var routing meshtastic.Routing
		if err := proto.Unmarshal(decoded.GetPayload(), &routing); err == nil {
			route := routing.GetRouteReply()
			if route == nil {
				route = routing.GetRouteRequest()
			}
			if route != nil {
				event.Type = EventTraceRoute
				event.TraceRoute = decodeTraceRoute(decoded, packet, route)
			}
		}
	default:
		event.Type = EventPacket
	}

	return event
}

func decodeTraceRoute(decoded *meshtastic.Data, packet *meshtastic.MeshPacket, route *meshtastic.RouteDiscovery) *TraceRoute {
	if route == nil {
		return nil
	}
	requestID := decoded.GetReplyId()
	if requestID == 0 {
		requestID = decoded.GetRequestId()
	}
	if requestID == 0 {
		requestID = packet.GetId()
	}
	return &TraceRoute{
		RequestID:  requestID,
		From:       packet.GetFrom(),
		To:         packet.GetTo(),
		Route:      append([]uint32(nil), route.GetRoute()...),
		SNRTowards: append([]int32(nil), route.GetSnrTowards()...),
		RouteBack:  append([]uint32(nil), route.GetRouteBack()...),
		SNRBack:    append([]int32(nil), route.GetSnrBack()...),
		RxRSSI:     int32ValuePtr(packet.GetRxRssi()),
		RxSNR:      float32ValuePtr(packet.GetRxSnr()),
	}
}

func int32ValuePtr(value int32) *int32 {
	copied := value
	return &copied
}

func float32ValuePtr(value float32) *float32 {
	copied := value
	return &copied
}
