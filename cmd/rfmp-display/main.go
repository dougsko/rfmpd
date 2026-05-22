package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "http://localhost:8080", "rfmpd address")
	flag.Parse()

	display := newDisplay()
	if err := display.Init(); err != nil {
		log.Fatalf("display init: %v", err)
	}
	defer display.Close()

	kb := newKeyboard()
	if err := kb.Init(); err != nil {
		log.Fatalf("keyboard init: %v", err)
	}
	defer kb.Close()

	led := newLED()
	if err := led.Init(); err != nil {
		log.Fatalf("led init: %v", err)
	}
	defer led.Close()

	client := NewClient(*addr)
	ws := NewWSClient(*addr)
	ws.Connect()
	defer ws.Close()

	app := &App{
		client: client,
		ws:     ws,
		led:    led,
	}

	status, err := client.GetStatus()
	if err == nil {
		app.author = status.Callsign
	} else {
		log.Printf("warning: could not reach rfmpd at %s: %v", *addr, err)
	}

	fb := NewFramebuffer(ScreenW, ScreenH)
	app.current = NewChannelListScreen(app)
	app.current.OnEnter()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			return
		default:
		}

		if !kb.Poll() {
			break
		}

	drainKeys:
		for {
			select {
			case ev := <-kb.Events():
				next := app.current.HandleKey(ev)
				if next != app.current {
					app.current = next
					app.current.OnEnter()
				}
			default:
				break drainKeys
			}
		}

	drainWS:
		for {
			select {
			case msg := <-ws.Messages():
				app.onNewMessage(msg)
			default:
				break drainWS
			}
		}

		fb.Clear(ColorBlack)
		app.renderStatusBar(fb)
		app.current.Render(fb)

		if led.IsOn() {
			fb.FillRect(ScreenW-20, 2, 6, 6, ColorGreen)
		}

		display.Flush(fb.Pix, fb.W, fb.H)

		<-ticker.C
	}
}
