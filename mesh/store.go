package mesh

import (
	"context"
	"sync"
	"time"
)

type Store interface {
	SaveMessage(context.Context, Message) error
	SaveNode(context.Context, Node) error
	SaveChannel(context.Context, Channel) error
	Messages(context.Context) ([]Message, error)
	Nodes(context.Context) ([]Node, error)
	Channels(context.Context) ([]Channel, error)
}

type MemoryStore struct {
	mu       sync.RWMutex
	messages []Message
	nodes    map[uint32]Node
	channels map[int32]Channel
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:    make(map[uint32]Node),
		channels: make(map[int32]Channel),
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

	s.nodes[node.Num] = node
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
