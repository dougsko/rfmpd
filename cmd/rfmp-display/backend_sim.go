//go:build sim

package main

import (
	"embed"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed sim.html
var simHTML embed.FS

const simPort = 9090

// SimDisplay serves a web page with a canvas that renders the framebuffer.
type SimDisplay struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
	upgrader websocket.Upgrader
}

func newDisplay() Display {
	return &SimDisplay{
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (d *SimDisplay) Init() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := simHTML.ReadFile("sim.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
	mux.HandleFunc("/ws", d.handleWS)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", simPort), mux); err != nil {
			log.Fatalf("Simulator HTTP server failed: %v", err)
		}
	}()
	log.Printf("Simulator UI: http://localhost:%d", simPort)
	return nil
}

func (d *SimDisplay) Flush(pix []uint16, w, h int) error {
	buf := make([]byte, len(pix)*2)
	for i, p := range pix {
		binary.LittleEndian.PutUint16(buf[i*2:], p)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for conn := range d.clients {
		err := conn.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			conn.Close()
			delete(d.clients, conn)
		}
	}
	return nil
}

func (d *SimDisplay) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for conn := range d.clients {
		conn.Close()
	}
}

func (d *SimDisplay) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	d.mu.Lock()
	d.clients[conn] = true
	d.mu.Unlock()

	// Read keyboard events from the browser
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			d.mu.Lock()
			delete(d.clients, conn)
			d.mu.Unlock()
			return
		}
		if simKB != nil && len(msg) > 0 {
			simKB.handleBrowserKey(msg)
		}
	}
}

// simKB is set during Init so the display WS handler can push key events.
var simKB *SimKeyboard

// SimKeyboard receives key events from the browser via WebSocket.
type SimKeyboard struct {
	events chan KeyEvent
}

func newKeyboard() Keyboard {
	kb := &SimKeyboard{
		events: make(chan KeyEvent, 32),
	}
	simKB = kb
	return kb
}

func (kb *SimKeyboard) Init() error { return nil }

func (kb *SimKeyboard) Events() <-chan KeyEvent {
	return kb.events
}

func (kb *SimKeyboard) Poll() bool {
	// No polling needed — keys arrive via WebSocket asynchronously
	return true
}

func (kb *SimKeyboard) Close() {}

func (kb *SimKeyboard) handleBrowserKey(msg []byte) {
	key := string(msg)
	var ev KeyEvent

	switch key {
	case "Enter":
		ev = KeyEvent{Code: KeyEnter}
	case "Escape":
		ev = KeyEvent{Code: KeyBack}
	case "Backspace":
		ev = KeyEvent{Code: KeyBackspace}
	case "ArrowUp":
		ev = KeyEvent{Code: KeyUp}
	case "ArrowDown":
		ev = KeyEvent{Code: KeyDown}
	case "ArrowLeft":
		ev = KeyEvent{Code: KeyLeft}
	case "ArrowRight":
		ev = KeyEvent{Code: KeyRight}
	case "Tab":
		ev = KeyEvent{Code: KeySym}
	default:
		if len(key) == 1 {
			ev = KeyEvent{Key: rune(key[0])}
		} else {
			runes := []rune(key)
			if len(runes) == 1 {
				ev = KeyEvent{Key: runes[0]}
			} else {
				return
			}
		}
	}

	select {
	case kb.events <- ev:
	default:
	}
}

// SimLED shows LED state.
type SimLED struct {
	mu sync.Mutex
	on bool
}

func newLED() LED {
	return &SimLED{}
}

func (l *SimLED) Init() error { return nil }

func (l *SimLED) On() {
	l.mu.Lock()
	l.on = true
	l.mu.Unlock()
}

func (l *SimLED) Off() {
	l.mu.Lock()
	l.on = false
	l.mu.Unlock()
}

func (l *SimLED) IsOn() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.on
}

func (l *SimLED) Blink(n int, interval time.Duration) {
	go func() {
		for i := 0; i < n; i++ {
			l.On()
			time.Sleep(interval)
			l.Off()
			time.Sleep(interval)
		}
	}()
}

func (l *SimLED) Close() {}
