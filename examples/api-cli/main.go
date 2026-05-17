package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"meshin/mesh"
)

type client struct {
	baseURL string
	http    *http.Client
}

type apiError struct {
	Error string `json:"error"`
}

type sendRequest struct {
	Text    string `json:"text"`
	Channel string `json:"channel,omitempty"`
	To      string `json:"to,omitempty"`
	WantAck bool   `json:"wantAck,omitempty"`
}

type sendResponse struct {
	ID uint32 `json:"id"`
}

type traceRequest struct {
	To             string `json:"to"`
	Channel        string `json:"channel,omitempty"`
	HopLimit       uint32 `json:"hopLimit,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type eventEnvelope struct {
	Type string          `json:"type"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}

func main() {
	apiURL := flag.String("api", "http://127.0.0.1:8080", "gomeshind API base URL")
	send := flag.String("send", "", "send a text message")
	channel := flag.String("channel", "", "channel name")
	to := flag.String("to", "", "destination node like !12345678")
	wantAck := flag.Bool("ack", false, "request an ACK when sending")
	listen := flag.Bool("listen", false, "listen for live messages")
	listNodes := flag.Bool("nodes", false, "list known nodes")
	listChannels := flag.Bool("channels", false, "list known channels")
	listMessages := flag.Bool("messages", false, "list stored messages")
	traceTo := flag.String("traceroute", "", "run traceroute to node like !12345678")
	timeout := flag.Int("timeout", 90, "traceroute timeout in seconds")
	flag.Parse()

	api, err := newClient(*apiURL)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch {
	case *listNodes:
		mustPrintNodes(ctx, api)
	case *listChannels:
		mustPrintChannels(ctx, api)
	case *listMessages:
		mustPrintMessages(ctx, api, *channel)
	case *send != "":
		mustSend(ctx, api, *send, *channel, *to, *wantAck)
	case *traceTo != "":
		mustTraceRoute(ctx, api, *traceTo, *channel, *timeout)
	case *listen:
		if err := api.listen(ctx, *channel, printLiveMessage); err != nil && ctx.Err() == nil {
			log.Fatal(err)
		}
	default:
		flag.Usage()
	}
}

func newClient(baseURL string) (*client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("api URL is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, err
	}
	return &client{baseURL: baseURL, http: &http.Client{}}, nil
}

func mustSend(ctx context.Context, api *client, text string, channel string, to string, wantAck bool) {
	var response sendResponse
	err := api.doJSON(ctx, http.MethodPost, "/messages", sendRequest{
		Text:    text,
		Channel: channel,
		To:      normalizeNode(to),
		WantAck: wantAck,
	}, &response)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sent id=%08x\n", response.ID)
}

func mustPrintNodes(ctx context.Context, api *client) {
	var nodes []mesh.Node
	if err := api.doJSON(ctx, http.MethodGet, "/nodes", nil, &nodes); err != nil {
		log.Fatal(err)
	}
	for _, node := range nodes {
		fmt.Printf("!%08x  %-8s  %s\n", node.Num, node.ShortName, node.LongName)
	}
}

func mustPrintChannels(ctx context.Context, api *client) {
	var channels []mesh.Channel
	if err := api.doJSON(ctx, http.MethodGet, "/channels", nil, &channels); err != nil {
		log.Fatal(err)
	}
	for _, channel := range channels {
		fmt.Printf("[%d] %-10s %s\n", channel.Index, channel.Role, displayChannel(channel.Name))
	}
}

func mustPrintMessages(ctx context.Context, api *client, channel string) {
	path := "/messages"
	if channel != "" {
		path += "?channel=" + url.QueryEscape(channel)
	}

	var messages []mesh.Message
	if err := api.doJSON(ctx, http.MethodGet, path, nil, &messages); err != nil {
		log.Fatal(err)
	}
	for _, message := range messages {
		printMessage(message)
	}
}

func mustTraceRoute(ctx context.Context, api *client, to string, channel string, timeoutSeconds int) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds+5)*time.Second)
	defer cancel()

	var route mesh.TraceRoute
	err := api.doJSON(ctx, http.MethodPost, "/traceroute", traceRequest{
		To:             normalizeNode(to),
		Channel:        channel,
		HopLimit:       3,
		TimeoutSeconds: timeoutSeconds,
	}, &route)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("traceroute id=%08x from=!%08x to=!%08x\n", route.RequestID, route.From, route.To)
	fmt.Println("towards:", formatTraceHops(route.Towards))
	if len(route.Back) > 0 {
		fmt.Println("back:   ", formatTraceHops(route.Back))
	}
}

func (c *client) doJSON(ctx context.Context, method string, path string, body interface{}, target interface{}) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("api returned %s", resp.Status)
	}

	if target == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *client) listen(ctx context.Context, channel string, handle func(mesh.Message)) error {
	path := "/events"
	if channel != "" {
		path += "?channel=" + url.QueryEscape(channel)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("events returned %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if data.Len() > 0 {
				if err := deliverEvent(eventType, data.String(), handle); err != nil {
					return err
				}
				eventType = ""
				data.Reset()
			}
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return scanner.Err()
}

func deliverEvent(eventType string, data string, handle func(mesh.Message)) error {
	if eventType != "" && eventType != "message.received" {
		return nil
	}

	var envelope eventEnvelope
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return err
	}
	if envelope.Type != "message.received" {
		return nil
	}

	var message mesh.Message
	if err := json.Unmarshal(envelope.Data, &message); err != nil {
		return err
	}
	handle(message)
	return nil
}

func printLiveMessage(message mesh.Message) {
	printMessage(message)
}

func printMessage(message mesh.Message) {
	from := fmt.Sprintf("!%08x", message.From.Num)
	if message.From.ShortName != "" {
		from = message.From.ShortName
	}
	fmt.Printf("[%s] %s: %s\n", displayChannel(message.Channel.Name), from, message.Text)
}

func formatTraceHops(hops []mesh.TraceHop) string {
	if len(hops) == 0 {
		return "(empty)"
	}

	parts := make([]string, 0, len(hops))
	for _, hop := range hops {
		label := hop.Node.ShortName
		if label == "" {
			label = fmt.Sprintf("!%08x", hop.Node.Num)
		}
		if hop.SNR != nil {
			label = fmt.Sprintf("%s %.1fdB", label, *hop.SNR)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " -> ")
}

func normalizeNode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "!")
	if _, err := strconv.ParseUint(value, 16, 32); err != nil {
		return value
	}
	return "!" + strings.ToLower(value)
}

func displayChannel(name string) string {
	if name == "" {
		return "Primary"
	}
	return name
}
