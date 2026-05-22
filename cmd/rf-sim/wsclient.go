package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	port int
	conn *websocket.Conn

	mu       sync.Mutex
	messages []Message
	closed   bool
}

type wsEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewWSClient(apiPort int) (*WSClient, error) {
	url := fmt.Sprintf("ws://127.0.0.1:%d/stream", apiPort)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket connect to port %d: %w", apiPort, err)
	}

	client := &WSClient{
		port: apiPort,
		conn: conn,
	}
	go client.readLoop()
	return client, nil
}

func (c *WSClient) readLoop() {
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
			return
		}

		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if env.Type == "message" {
			var msg Message
			if err := json.Unmarshal(env.Data, &msg); err == nil {
				c.mu.Lock()
				c.messages = append(c.messages, msg)
				c.mu.Unlock()
			}
		}
	}
}

func (c *WSClient) GetMessages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Message, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *WSClient) GetMessageIDs() map[string]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make(map[string]bool, len(c.messages))
	for _, m := range c.messages {
		ids[m.ID] = true
	}
	return ids
}

func (c *WSClient) MessageCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.messages)
}

func (c *WSClient) Close() {
	c.conn.Close()
}

func (c *WSClient) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func WaitWSMessages(clients []*WSClient, expectedCount int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allReady := true
		for _, c := range clients {
			if c.MessageCount() < expectedCount {
				allReady = false
				break
			}
		}
		if allReady {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	var details []string
	for i, c := range clients {
		details = append(details, fmt.Sprintf("client%d: %d msgs", i, c.MessageCount()))
	}
	return fmt.Errorf("WS timeout (expected %d): %s", expectedCount, joinDetails(details))
}

func joinDetails(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
