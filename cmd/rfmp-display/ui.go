package main

import (
	"fmt"
	"time"
)

const (
	ScreenW    = 320
	ScreenH    = 240
	FontW      = 8
	FontH      = 16
	Cols       = ScreenW / FontW // 40
	Rows       = ScreenH / FontH // 15
	StatusBarH = FontH
)

type Screen interface {
	Render(fb *Framebuffer)
	HandleKey(ev KeyEvent) Screen
	OnEnter()
}

type App struct {
	client    *Client
	ws        *WSClient
	led       LED
	current   Screen
	channels  []Channel
	messages  []Message
	channel   string
	author    string
	err       string
}

func (app *App) renderStatusBar(fb *Framebuffer) {
	fb.FillRect(0, 0, ScreenW, StatusBarH, ColorDarkGray)

	label := "RFMP"
	if app.channel != "" {
		label = fmt.Sprintf("# %s", app.channel)
	}
	if len(label) > Cols-2 {
		label = label[:Cols-2]
	}
	fb.DrawString(4, 0, label, ColorWhite, ColorDarkGray)

	dotColor := ColorRed
	if app.ws.Connected() {
		dotColor = ColorGreen
	}
	fb.FillRect(ScreenW-12, 4, 8, 8, dotColor)
}

func (app *App) onNewMessage(msg Message) {
	if msg.Channel == app.channel {
		for _, m := range app.messages {
			if m.ID == msg.ID {
				return
			}
		}
		app.messages = append([]Message{msg}, app.messages...)
		app.led.Blink(3, 100*time.Millisecond)
	}
}
