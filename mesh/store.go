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
	SaveChannel(context.Context, Channel) error
	Messages(context.Context) ([]Message, error)
	Nodes(context.Context) ([]Node, error)
	Positions(context.Context) ([]Position, error)
	EnvironmentTelemetries(context.Context) ([]EnvironmentTelemetry, error)
	Channels(context.Context) ([]Channel, error)
}

type MemoryStore struct {
	mu           sync.RWMutex
	messages     []Message
	nodes        map[uint32]Node
	positions    map[uint32]Position
	environments map[uint32]EnvironmentTelemetry
	channels     map[int32]Channel
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:        make(map[uint32]Node),
		positions:    make(map[uint32]Position),
		environments: make(map[uint32]EnvironmentTelemetry),
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
