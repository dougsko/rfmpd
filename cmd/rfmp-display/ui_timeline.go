package main

import (
	"fmt"
	"time"
)

type TimelineScreen struct {
	app    *App
	scroll int
}

func NewTimelineScreen(app *App) *TimelineScreen {
	return &TimelineScreen{app: app}
}

func (s *TimelineScreen) OnEnter() {
	messages, err := s.app.client.GetMessages(s.app.channel, 50)
	if err == nil {
		s.app.messages = messages
	}
	s.scroll = 0
}

func (s *TimelineScreen) Render(fb *Framebuffer) {
	y := StatusBarH

	if len(s.app.messages) == 0 {
		fb.DrawString(4, y+FontH, "No messages yet", ColorLightGray, ColorBlack)
		fb.DrawString(4, y+FontH*2, "Press any key to compose", ColorLightGray, ColorBlack)
		return
	}

	msgIdx := s.scroll
	for msgIdx < len(s.app.messages) && y < ScreenH {
		msg := s.app.messages[msgIdx]
		y = s.renderMessage(fb, y, msg)
		msgIdx++
	}
}

func (s *TimelineScreen) renderMessage(fb *Framebuffer, y int, msg Message) int {
	if y >= ScreenH {
		return y
	}

	fb.HLine(0, y, ScreenW, ColorDarkGray)
	y++

	author := msg.FromNode
	if msg.Author != nil && *msg.Author != "" {
		author = *msg.Author
	}
	ts := formatTimestamp(msg.Timestamp)

	header := author
	if len(header) > Cols-8 {
		header = header[:Cols-8]
	}
	fb.DrawString(4, y, header, ColorBlue, ColorBlack)

	tsX := ScreenW - (len(ts)+1)*FontW
	if tsX > 0 {
		fb.DrawString(tsX, y, ts, ColorLightGray, ColorBlack)
	}
	y += FontH

	if y >= ScreenH {
		return y
	}
	lines := fb.DrawStringWrap(4, y, ScreenW-8, msg.Body, ColorWhite, ColorBlack)
	y += lines * FontH

	y += 2
	return y
}

func (s *TimelineScreen) HandleKey(ev KeyEvent) Screen {
	switch ev.Code {
	case KeyDown:
		if s.scroll < len(s.app.messages)-1 {
			s.scroll++
		}
	case KeyUp:
		if s.scroll > 0 {
			s.scroll--
		}
	case KeyBack:
		return NewChannelListScreen(s.app)
	}

	if ev.Key != 0 {
		return NewComposeScreen(s.app, ev.Key)
	}
	return s
}

func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("20060102T150405Z", ts)
		if err != nil {
			return ts
		}
	}

	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
	}
	return fmt.Sprintf("%d/%d %02d:%02d", t.Month(), t.Day(), t.Hour(), t.Minute())
}
