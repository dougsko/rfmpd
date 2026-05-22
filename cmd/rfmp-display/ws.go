package main

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	url       string
	messages  chan Message
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	connected bool
}

type wsEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewWSClient(baseURL string) *WSClient {
	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/stream"

	return &WSClient{
		url:      wsURL,
		messages: make(chan Message, 64),
		done:     make(chan struct{}),
	}
}

func (ws *WSClient) Connect() {
	go ws.connectLoop()
}

func (ws *WSClient) Messages() <-chan Message {
	return ws.messages
}

func (ws *WSClient) Connected() bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.connected
}

func (ws *WSClient) setConnected(v bool) {
	ws.mu.Lock()
	ws.connected = v
	ws.mu.Unlock()
}

func (ws *WSClient) Close() {
	ws.closeOnce.Do(func() {
		close(ws.done)
	})
}

func (ws *WSClient) connectLoop() {
	backoff := time.Second

	for {
		select {
		case <-ws.done:
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(ws.url, nil)
		if err != nil {
			log.Printf("ws: connect failed: %v (retry in %v)", err, backoff)
			select {
			case <-time.After(backoff):
			case <-ws.done:
				return
			}
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}

		backoff = time.Second
		ws.setConnected(true)
		ws.readLoop(conn)
		ws.setConnected(false)
		conn.Close()
	}
}

func (ws *WSClient) readLoop(conn *websocket.Conn) {
	for {
		select {
		case <-ws.done:
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ws: read error: %v (reconnecting)", err)
			return
		}

		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if env.Type == "message" {
			var msg Message
			if err := json.Unmarshal(env.Data, &msg); err == nil {
				select {
				case ws.messages <- msg:
				default:
				}
			}
		}
	}
}
