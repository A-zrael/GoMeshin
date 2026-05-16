package sqlitestore

import (
	"context"
	"testing"
	"time"

	"meshin/mesh"
)

func TestStorePersistsMessagesNodesAndChannels(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir()+"/gomeshin.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	node := mesh.Node{
		Num:       0x12345678,
		ID:        "!12345678",
		LongName:  "Long Name",
		ShortName: "LN",
		LastSeen:  time.Unix(100, 0),
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}

	channel := mesh.Channel{
		Index:    1,
		Name:     "ops",
		Role:     "SECONDARY",
		ID:       42,
		PSKBytes: 1,
	}
	if err := store.SaveChannel(ctx, channel); err != nil {
		t.Fatal(err)
	}

	message := mesh.Message{
		ID:   99,
		From: mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		To:   0xffffffff,
		Channel: mesh.ChannelRef{
			Index: channel.Index,
			Name:  channel.Name,
		},
		Text:       "hello",
		RSSI:       -80,
		SNR:        7.5,
		ReceivedAt: time.Unix(123, 0),
	}
	if err := store.SaveMessage(ctx, message); err != nil {
		t.Fatal(err)
	}

	messages, err := store.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Text != "hello" {
		t.Fatalf("unexpected messages: %#v", messages)
	}

	nodes, err := store.Nodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ShortName != "LN" {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}

	channels, err := store.Channels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Name != "ops" {
		t.Fatalf("unexpected channels: %#v", channels)
	}
}
