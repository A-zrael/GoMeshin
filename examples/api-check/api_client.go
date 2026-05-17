package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"meshin/mesh"
)

type apiClient struct {
	baseURL string
	http    *http.Client
}

type apiError struct {
	Error string `json:"error"`
}

type sendAPIRequest struct {
	Text    string `json:"text"`
	Channel string `json:"channel,omitempty"`
	WantAck bool   `json:"wantAck,omitempty"`
}

type sendAPIResponse struct {
	ID uint32 `json:"id"`
}

type channelAPIRequest struct {
	Name string `json:"name"`
}

type traceAPIRequest struct {
	To             uint32 `json:"to"`
	Channel        string `json:"channel,omitempty"`
	HopLimit       uint32 `json:"hopLimit,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type eventAPIEnvelope struct {
	Type string          `json:"type"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}

func newAPIClient(baseURL string) (*apiClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("api URL is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, err
	}
	return &apiClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}, nil
}

func (c *apiClient) Messages(ctx context.Context) ([]mesh.Message, error) {
	var messages []mesh.Message
	err := c.doJSON(ctx, http.MethodGet, "/messages", nil, &messages)
	return messages, err
}

func (c *apiClient) Nodes(ctx context.Context) ([]mesh.Node, error) {
	var nodes []mesh.Node
	err := c.doJSON(ctx, http.MethodGet, "/nodes", nil, &nodes)
	return nodes, err
}

func (c *apiClient) Channels(ctx context.Context) ([]mesh.Channel, error) {
	var channels []mesh.Channel
	err := c.doJSON(ctx, http.MethodGet, "/channels", nil, &channels)
	return channels, err
}

func (c *apiClient) Send(ctx context.Context, text string, opts mesh.SendOptions) (uint32, error) {
	var response sendAPIResponse
	err := c.doJSON(ctx, http.MethodPost, "/messages", sendAPIRequest{
		Text:    text,
		Channel: opts.Channel,
		WantAck: opts.WantAck,
	}, &response)
	return response.ID, err
}

func (c *apiClient) AddChannel(ctx context.Context, name string) ([]mesh.Channel, error) {
	var channels []mesh.Channel
	err := c.doJSON(ctx, http.MethodPost, "/channels", channelAPIRequest{Name: name}, &channels)
	return channels, err
}

func (c *apiClient) RemoveChannel(ctx context.Context, name string) ([]mesh.Channel, error) {
	path := "/channels/" + url.PathEscape(name)
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return nil, err
	}
	return c.Channels(ctx)
}

func (c *apiClient) TraceRoute(ctx context.Context, opts mesh.TraceRouteOptions) (mesh.TraceRoute, error) {
	var route mesh.TraceRoute
	err := c.doJSON(ctx, http.MethodPost, "/traceroute", traceAPIRequest{
		To:             opts.To,
		Channel:        opts.Channel,
		HopLimit:       opts.HopLimit,
		TimeoutSeconds: 90,
	}, &route)
	return route, err
}

func (c *apiClient) Events(ctx context.Context) <-chan mesh.Message {
	out := make(chan mesh.Message, 64)
	go func() {
		defer close(out)
		for {
			if ctx.Err() != nil {
				return
			}
			if err := c.readEvents(ctx, out); err != nil && ctx.Err() == nil {
				time.Sleep(2 * time.Second)
			}
		}
	}()
	return out
}

func (c *apiClient) doJSON(ctx context.Context, method string, path string, body interface{}, target interface{}) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
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

func (c *apiClient) readEvents(ctx context.Context, out chan<- mesh.Message) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/events", nil)
	if err != nil {
		return err
	}

	client := *c.http
	client.Timeout = 0
	resp, err := client.Do(req)
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
				if err := deliverEvent(eventType, data.String(), out); err != nil {
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

func deliverEvent(eventType string, data string, out chan<- mesh.Message) error {
	if eventType != "" && eventType != "message.received" {
		return nil
	}

	var envelope eventAPIEnvelope
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
	out <- message
	return nil
}
