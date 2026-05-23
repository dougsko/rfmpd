package main

import (
	"fmt"
	"log"
)

type ChannelListScreen struct {
	app      *App
	selected int
	scroll   int
}

func NewChannelListScreen(app *App) *ChannelListScreen {
	return &ChannelListScreen{app: app}
}

func (s *ChannelListScreen) OnEnter() {
	s.app.channel = ""
	channels, err := s.app.client.GetChannels()
	if err != nil {
		log.Printf("GetChannels error: %v", err)
		s.app.err = err.Error()
	} else {
		log.Printf("GetChannels: got %d channels", len(channels))
		s.app.channels = channels
		s.app.err = ""
	}
	if s.selected >= len(s.app.channels) {
		s.selected = 0
	}
}

func (s *ChannelListScreen) Render(fb *Framebuffer) {
	y := StatusBarH + 4
	fb.DrawString(4, y, "Channels", ColorBlue, ColorBlack)
	y += FontH + 4

	fb.HLine(0, y, ScreenW, ColorDarkGray)
	y += 2

	visibleItems := (ScreenH - y) / FontH
	if s.scroll > s.selected {
		s.scroll = s.selected
	}
	if s.selected >= s.scroll+visibleItems {
		s.scroll = s.selected - visibleItems + 1
	}

	for i := s.scroll; i < len(s.app.channels) && y+FontH <= ScreenH; i++ {
		ch := s.app.channels[i]
		bg := ColorBlack
		fg := ColorWhite
		if i == s.selected {
			bg = ColorDarkGray
			fg = ColorWhite
			fb.FillRect(0, y, ScreenW, FontH, bg)
		}

		label := fmt.Sprintf("# %-20s %3d", ch.Name, ch.MessageCount)
		if len(label) > Cols {
			label = label[:Cols]
		}
		fb.DrawString(4, y, label, fg, bg)
		y += FontH
	}

	if len(s.app.channels) == 0 {
		fb.DrawString(4, y, "No channels found", ColorLightGray, ColorBlack)
	}

	if s.app.err != "" {
		fb.DrawStringWrap(4, ScreenH-FontH*2, ScreenW-8, s.app.err, ColorRed, ColorBlack)
	}
	fb.DrawString(4, ScreenH-FontH, "n:new d:del s:settings", ColorLightGray, ColorBlack)
}

func (s *ChannelListScreen) HandleKey(ev KeyEvent) Screen {
	switch ev.Code {
	case KeyDown:
		if s.selected < len(s.app.channels)-1 {
			s.selected++
		}
	case KeyUp:
		if s.selected > 0 {
			s.selected--
		}
	case KeyEnter:
		if len(s.app.channels) > 0 {
			s.app.channel = s.app.channels[s.selected].Name
			return NewTimelineScreen(s.app)
		}
	}

	switch ev.Key {
	case 's':
		return NewSettingsScreen(s.app)
	case 'n':
		return NewCreateChannelScreen(s.app)
	case 'd':
		if len(s.app.channels) > 0 {
			ch := s.app.channels[s.selected]
			if ch.MessageCount == 0 {
				if err := s.app.client.DeleteChannel(ch.Name); err != nil {
					log.Printf("DeleteChannel error: %v", err)
					s.app.err = "delete: " + err.Error()
				} else {
					s.app.err = ""
				}
				s.OnEnter()
			} else {
				s.app.err = "can't delete: has messages"
			}
		}
	}
	return s
}
