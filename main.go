package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"meshin/mesh"
	"meshin/mesh/sqlitestore"
)

func main() {
	device := flag.String("port", "/dev/ttyUSB0", "serial port connected to the Meshtastic radio")
	baud := flag.Int("baud", 115200, "serial baud rate")
	send := flag.String("send", "", "send a text message and exit")
	channel := flag.String("channel", "", "channel name to use when sending")
	listChannels := flag.Bool("channels", false, "print known channels and exit")
	listNodes := flag.Bool("nodes", false, "print known nodes and exit")
	interactive := flag.Bool("i", false, "read text from stdin and send each line")
	dbPath := flag.String("db", "", "optional SQLite database path for persistent storage")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var store mesh.Store
	if *dbPath != "" {
		sqliteStore, err := sqlitestore.Open(ctx, *dbPath)
		if err != nil {
			log.Fatal(err)
		}
		defer sqliteStore.Close()
		store = sqliteStore
	}

	m, err := mesh.Open(ctx, mesh.Config{
		Port:  *device,
		Baud:  *baud,
		Store: store,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()

	if *listChannels {
		printChannels(ctx, m)
		return
	}

	if *listNodes {
		printNodes(ctx, m)
		return
	}

	messages, unsubscribe := m.Subscribe(64)
	defer unsubscribe()
	go printMessages(messages)

	if *send != "" {
		id, err := m.Send(*send, mesh.SendOptions{Channel: *channel})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("SENT id=%08x channel=%q text=%q\n", id, channelName(*channel), *send)
		return
	}

	if *interactive {
		sendLines(m, *channel)
		return
	}

	select {}
}

func printMessages(messages <-chan mesh.Message) {
	for message := range messages {
		from := fmt.Sprintf("!%08x", message.From.Num)
		if message.From.ShortName != "" {
			from = message.From.ShortName
		}

		fmt.Printf("[%s] %s: %s\n", channelName(message.Channel.Name), from, message.Text)
	}
}

func sendLines(m *mesh.Mesh, channel string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		id, err := m.Send(text, mesh.SendOptions{Channel: channel})
		if err != nil {
			fmt.Printf("SEND_ERROR %v\n", err)
			continue
		}

		fmt.Printf("SENT id=%08x channel=%q text=%q\n", id, channelName(channel), text)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("STDIN_ERROR %v\n", err)
	}
}

func printChannels(ctx context.Context, m *mesh.Mesh) {
	channels, err := m.Channels(ctx)
	if err != nil {
		fmt.Printf("CHANNEL_ERROR %v\n", err)
		return
	}
	if len(channels) == 0 {
		fmt.Println("CHANNELS none known")
		return
	}

	for _, channel := range channels {
		fmt.Printf("CHANNEL index=%d role=%s name=%q id=%08x psk_bytes=%d\n",
			channel.Index, channel.Role, channelName(channel.Name), channel.ID, channel.PSKBytes)
	}
}

func printNodes(ctx context.Context, m *mesh.Mesh) {
	nodes, err := m.Nodes(ctx)
	if err != nil {
		fmt.Printf("NODE_ERROR %v\n", err)
		return
	}
	if len(nodes) == 0 {
		fmt.Println("NODES none known")
		return
	}

	for _, node := range nodes {
		fmt.Printf("NODE num=!%08x id=%s short=%q long=%q\n", node.Num, node.ID, node.ShortName, node.LongName)
	}
}

func channelName(name string) string {
	if name == "" {
		return "Primary"
	}
	return name
}
