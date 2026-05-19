package mesh

import (
	"context"
	"sync"
	"time"
)

type Store interface {
	SaveMessage(context.Context, Message) error
	SaveNode(context.Context, Node) error
	SavePosition(context.Context, Position) error
	SaveEnvironmentTelemetry(context.Context, EnvironmentTelemetry) error
	SaveDeviceTelemetry(context.Context, DeviceTelemetry) error
	SavePowerTelemetry(context.Context, PowerTelemetry) error
	SaveAirQualityTelemetry(context.Context, AirQualityTelemetry) error
	SaveLocalStatsTelemetry(context.Context, LocalStatsTelemetry) error
	SaveHealthTelemetry(context.Context, HealthTelemetry) error
	SaveTraceRoute(context.Context, TraceRoute) error
	SaveChannel(context.Context, Channel) error
	Messages(context.Context) ([]Message, error)
	Nodes(context.Context) ([]Node, error)
	Positions(context.Context) ([]Position, error)
	EnvironmentTelemetries(context.Context) ([]EnvironmentTelemetry, error)
	DeviceTelemetries(context.Context) ([]DeviceTelemetry, error)
	PowerTelemetries(context.Context) ([]PowerTelemetry, error)
	AirQualityTelemetries(context.Context) ([]AirQualityTelemetry, error)
	LocalStatsTelemetries(context.Context) ([]LocalStatsTelemetry, error)
	HealthTelemetries(context.Context) ([]HealthTelemetry, error)
	EnvironmentTelemetryHistory(context.Context, uint32, int) ([]EnvironmentTelemetry, error)
	DeviceTelemetryHistory(context.Context, uint32, int) ([]DeviceTelemetry, error)
	LocalStatsTelemetryHistory(context.Context, uint32, int) ([]LocalStatsTelemetry, error)
	TraceRoutes(context.Context, uint32, int) ([]TraceRoute, error)
	Channels(context.Context) ([]Channel, error)
}

type MemoryStore struct {
	mu                 sync.RWMutex
	messages           []Message
	nodes              map[uint32]Node
	positions          map[uint32]Position
	environments       map[uint32]EnvironmentTelemetry
	devices            map[uint32]DeviceTelemetry
	powers             map[uint32]PowerTelemetry
	airs               map[uint32]AirQualityTelemetry
	locals             map[uint32]LocalStatsTelemetry
	healths            map[uint32]HealthTelemetry
	environmentHistory []EnvironmentTelemetry
	deviceHistory      []DeviceTelemetry
	localStatsHistory  []LocalStatsTelemetry
	traceHistory       []TraceRoute
	channels           map[int32]Channel
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:        make(map[uint32]Node),
		positions:    make(map[uint32]Position),
		environments: make(map[uint32]EnvironmentTelemetry),
		devices:      make(map[uint32]DeviceTelemetry),
		powers:       make(map[uint32]PowerTelemetry),
		airs:         make(map[uint32]AirQualityTelemetry),
		locals:       make(map[uint32]LocalStatsTelemetry),
		healths:      make(map[uint32]HealthTelemetry),
		channels:     make(map[int32]Channel),
	}
}

func (s *MemoryStore) SaveMessage(_ context.Context, message Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = time.Now()
	}
	s.messages = append(s.messages, message)
	return nil
}

func (s *MemoryStore) SaveNode(_ context.Context, node Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if position, ok := s.positions[node.Num]; ok && node.Position == nil {
		node.Position = &position
	}
	if environment, ok := s.environments[node.Num]; ok && node.Environment == nil {
		node.Environment = &environment
	}
	s.nodes[node.Num] = node
	if node.Position != nil {
		s.positions[node.Num] = *node.Position
	}
	if node.Environment != nil {
		s.environments[node.Num] = *node.Environment
	}
	return nil
}

func (s *MemoryStore) SavePosition(_ context.Context, position Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if position.ReceivedAt.IsZero() {
		position.ReceivedAt = time.Now()
	}
	s.positions[position.Node.Num] = position
	return nil
}

func (s *MemoryStore) SaveEnvironmentTelemetry(_ context.Context, environment EnvironmentTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if environment.ReceivedAt.IsZero() {
		environment.ReceivedAt = time.Now()
	}
	s.environments[environment.Node.Num] = environment
	s.environmentHistory = append(s.environmentHistory, environment)
	return nil
}

func (s *MemoryStore) SaveDeviceTelemetry(_ context.Context, sample DeviceTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.ReceivedAt.IsZero() {
		sample.ReceivedAt = time.Now()
	}
	s.devices[sample.Node.Num] = sample
	s.deviceHistory = append(s.deviceHistory, sample)
	return nil
}

func (s *MemoryStore) SavePowerTelemetry(_ context.Context, sample PowerTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.ReceivedAt.IsZero() {
		sample.ReceivedAt = time.Now()
	}
	s.powers[sample.Node.Num] = sample
	return nil
}

func (s *MemoryStore) SaveAirQualityTelemetry(_ context.Context, sample AirQualityTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.ReceivedAt.IsZero() {
		sample.ReceivedAt = time.Now()
	}
	s.airs[sample.Node.Num] = sample
	return nil
}

func (s *MemoryStore) SaveLocalStatsTelemetry(_ context.Context, sample LocalStatsTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.ReceivedAt.IsZero() {
		sample.ReceivedAt = time.Now()
	}
	s.locals[sample.Node.Num] = sample
	s.localStatsHistory = append(s.localStatsHistory, sample)
	return nil
}

func (s *MemoryStore) SaveHealthTelemetry(_ context.Context, sample HealthTelemetry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.ReceivedAt.IsZero() {
		sample.ReceivedAt = time.Now()
	}
	s.healths[sample.Node.Num] = sample
	return nil
}

func (s *MemoryStore) SaveTraceRoute(_ context.Context, route TraceRoute) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if route.ReceivedAt.IsZero() {
		route.ReceivedAt = time.Now()
	}
	s.traceHistory = append(s.traceHistory, route)
	return nil
}

func (s *MemoryStore) SaveChannel(_ context.Context, channel Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels[channel.Index] = channel
	return nil
}

func (s *MemoryStore) Messages(_ context.Context) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]Message, len(s.messages))
	copy(messages, s.messages)
	return messages, nil
}

func (s *MemoryStore) Nodes(_ context.Context) ([]Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *MemoryStore) Positions(_ context.Context) ([]Position, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	positions := make([]Position, 0, len(s.positions))
	for _, position := range s.positions {
		positions = append(positions, position)
	}
	return positions, nil
}

func (s *MemoryStore) EnvironmentTelemetries(_ context.Context) ([]EnvironmentTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	environments := make([]EnvironmentTelemetry, 0, len(s.environments))
	for _, environment := range s.environments {
		environments = append(environments, environment)
	}
	return environments, nil
}

func (s *MemoryStore) DeviceTelemetries(_ context.Context) ([]DeviceTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DeviceTelemetry, 0, len(s.devices))
	for _, sample := range s.devices {
		out = append(out, sample)
	}
	return out, nil
}

func (s *MemoryStore) PowerTelemetries(_ context.Context) ([]PowerTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PowerTelemetry, 0, len(s.powers))
	for _, sample := range s.powers {
		out = append(out, sample)
	}
	return out, nil
}

func (s *MemoryStore) AirQualityTelemetries(_ context.Context) ([]AirQualityTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AirQualityTelemetry, 0, len(s.airs))
	for _, sample := range s.airs {
		out = append(out, sample)
	}
	return out, nil
}

func (s *MemoryStore) LocalStatsTelemetries(_ context.Context) ([]LocalStatsTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LocalStatsTelemetry, 0, len(s.locals))
	for _, sample := range s.locals {
		out = append(out, sample)
	}
	return out, nil
}

func (s *MemoryStore) HealthTelemetries(_ context.Context) ([]HealthTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HealthTelemetry, 0, len(s.healths))
	for _, sample := range s.healths {
		out = append(out, sample)
	}
	return out, nil
}

func (s *MemoryStore) EnvironmentTelemetryHistory(_ context.Context, nodeNum uint32, limit int) ([]EnvironmentTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterEnvironmentHistory(s.environmentHistory, nodeNum, limit), nil
}

func (s *MemoryStore) DeviceTelemetryHistory(_ context.Context, nodeNum uint32, limit int) ([]DeviceTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterDeviceHistory(s.deviceHistory, nodeNum, limit), nil
}

func (s *MemoryStore) LocalStatsTelemetryHistory(_ context.Context, nodeNum uint32, limit int) ([]LocalStatsTelemetry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterLocalStatsHistory(s.localStatsHistory, nodeNum, limit), nil
}

func (s *MemoryStore) TraceRoutes(_ context.Context, nodeNum uint32, limit int) ([]TraceRoute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterTraceHistory(s.traceHistory, nodeNum, limit), nil
}

func filterEnvironmentHistory(samples []EnvironmentTelemetry, nodeNum uint32, limit int) []EnvironmentTelemetry {
	if limit <= 0 {
		limit = 200
	}
	out := make([]EnvironmentTelemetry, 0, limit)
	for i := len(samples) - 1; i >= 0 && len(out) < limit; i-- {
		sample := samples[i]
		if nodeNum != 0 && sample.Node.Num != nodeNum {
			continue
		}
		out = append(out, sample)
	}
	return out
}

func filterDeviceHistory(samples []DeviceTelemetry, nodeNum uint32, limit int) []DeviceTelemetry {
	if limit <= 0 {
		limit = 200
	}
	out := make([]DeviceTelemetry, 0, limit)
	for i := len(samples) - 1; i >= 0 && len(out) < limit; i-- {
		sample := samples[i]
		if nodeNum != 0 && sample.Node.Num != nodeNum {
			continue
		}
		out = append(out, sample)
	}
	return out
}

func filterLocalStatsHistory(samples []LocalStatsTelemetry, nodeNum uint32, limit int) []LocalStatsTelemetry {
	if limit <= 0 {
		limit = 200
	}
	out := make([]LocalStatsTelemetry, 0, limit)
	for i := len(samples) - 1; i >= 0 && len(out) < limit; i-- {
		sample := samples[i]
		if nodeNum != 0 && sample.Node.Num != nodeNum {
			continue
		}
		out = append(out, sample)
	}
	return out
}

func filterTraceHistory(samples []TraceRoute, nodeNum uint32, limit int) []TraceRoute {
	if limit <= 0 {
		limit = 200
	}
	out := make([]TraceRoute, 0, limit)
	for i := len(samples) - 1; i >= 0 && len(out) < limit; i-- {
		sample := samples[i]
		if nodeNum != 0 && sample.From != nodeNum && sample.To != nodeNum {
			continue
		}
		out = append(out, sample)
	}
	return out
}

func (s *MemoryStore) Channels(_ context.Context) ([]Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]Channel, 0, len(s.channels))
	for _, channel := range s.channels {
		channels = append(channels, channel)
	}
	sortChannels(channels)
	return channels, nil
}

func sortChannels(channels []Channel) {
	for i := 0; i < len(channels); i++ {
		for j := i + 1; j < len(channels); j++ {
			if channels[j].Index < channels[i].Index {
				channels[i], channels[j] = channels[j], channels[i]
			}
		}
	}
}
