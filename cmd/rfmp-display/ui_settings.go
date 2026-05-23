package main

import (
	"fmt"
	"log"
	"strconv"
)

type settingField struct {
	label string
	value string
}

type SettingsScreen struct {
	app      *App
	fields   []settingField
	selected int
	scroll   int
	editing  bool
	editBuf  string
}

func NewSettingsScreen(app *App) *SettingsScreen {
	return &SettingsScreen{app: app}
}

func (s *SettingsScreen) OnEnter() {
	cfg, err := s.app.client.GetConfig()
	if err != nil {
		log.Printf("GetConfig error: %v", err)
		s.app.err = err.Error()
		s.fields = nil
		return
	}
	s.app.err = ""
	s.fields = []settingField{
		{"Callsign", cfg.Node.Callsign},
		{"SSID", strconv.Itoa(cfg.Node.SSID)},
		{"Direwolf Host", cfg.Network.DirewolfHost},
		{"Direwolf Port", strconv.Itoa(cfg.Network.DirewolfPort)},
		{"Sync Interval", strconv.Itoa(cfg.Sync.SyncInterval)},
		{"Log Level", cfg.Logging.Level},
	}
}

func (s *SettingsScreen) Render(fb *Framebuffer) {
	y := StatusBarH + 4
	fb.DrawString(4, y, "Settings", ColorBlue, ColorBlack)
	y += FontH + 4

	fb.HLine(0, y, ScreenW, ColorDarkGray)
	y += 2

	if s.fields == nil {
		fb.DrawString(4, y, "Loading...", ColorLightGray, ColorBlack)
		if s.app.err != "" {
			fb.DrawStringWrap(4, y+FontH+4, ScreenW-8, s.app.err, ColorRed, ColorBlack)
		}
		return
	}

	visibleItems := (ScreenH - y - FontH*2) / FontH
	if visibleItems < 1 {
		visibleItems = 1
	}
	if s.scroll > s.selected {
		s.scroll = s.selected
	}
	if s.selected >= s.scroll+visibleItems {
		s.scroll = s.selected - visibleItems + 1
	}

	for i := s.scroll; i < len(s.fields) && y+FontH <= ScreenH-FontH*2; i++ {
		f := s.fields[i]
		bg := ColorBlack
		fg := ColorWhite

		if i == s.selected {
			bg = ColorDarkGray
			fg = ColorWhite
			fb.FillRect(0, y, ScreenW, FontH, bg)
		}

		val := f.value
		if i == s.selected && s.editing {
			val = s.editBuf + "_"
			fg = ColorGreen
		}

		label := fmt.Sprintf("%-15s %s", f.label, val)
		if len(label) > Cols {
			label = label[:Cols]
		}
		fb.DrawString(4, y, label, fg, bg)
		y += FontH
	}

	if s.app.err != "" {
		fb.DrawStringWrap(4, ScreenH-FontH*2, ScreenW-8, s.app.err, ColorRed, ColorBlack)
	}

	help := "Enter:edit Bk:back S:save P:shutdown"
	if s.editing {
		help = "Enter:done Bk:cancel"
	}
	fb.DrawString(4, ScreenH-FontH, help, ColorLightGray, ColorBlack)
}

func (s *SettingsScreen) HandleKey(ev KeyEvent) Screen {
	if s.editing {
		return s.handleEditKey(ev)
	}

	switch ev.Code {
	case KeyDown:
		if s.selected < len(s.fields)-1 {
			s.selected++
		}
	case KeyUp:
		if s.selected > 0 {
			s.selected--
		}
	case KeyEnter:
		if s.fields != nil && s.selected < len(s.fields) {
			s.editing = true
			s.editBuf = s.fields[s.selected].value
		}
	case KeyBack:
		return NewChannelListScreen(s.app)
	}

	switch ev.Key {
	case 's', 'S':
		s.save()
	case 'p', 'P':
		s.shutdown()
	}
	return s
}

func (s *SettingsScreen) handleEditKey(ev KeyEvent) Screen {
	switch ev.Code {
	case KeyEnter:
		s.fields[s.selected].value = s.editBuf
		s.editing = false
	case KeyBack:
		s.editing = false
	case KeyBackspace:
		if len(s.editBuf) > 0 {
			s.editBuf = s.editBuf[:len(s.editBuf)-1]
		}
	default:
		if ev.Key >= 32 && ev.Key < 127 {
			s.editBuf += string(ev.Key)
		}
	}
	return s
}

func (s *SettingsScreen) save() {
	if s.fields == nil {
		return
	}

	cfg := &ConfigData{}
	for _, f := range s.fields {
		switch f.label {
		case "Callsign":
			cfg.Node.Callsign = f.value
		case "SSID":
			v, _ := strconv.Atoi(f.value)
			cfg.Node.SSID = v
		case "Direwolf Host":
			cfg.Network.DirewolfHost = f.value
		case "Direwolf Port":
			v, _ := strconv.Atoi(f.value)
			cfg.Network.DirewolfPort = v
		case "Sync Interval":
			v, _ := strconv.Atoi(f.value)
			cfg.Sync.SyncInterval = v
		case "Log Level":
			cfg.Logging.Level = f.value
		}
	}

	if err := s.app.client.SaveConfig(cfg); err != nil {
		log.Printf("SaveConfig error: %v", err)
		s.app.err = "save: " + err.Error()
	} else {
		s.app.err = "saved, restarting..."
	}
}

func (s *SettingsScreen) shutdown() {
	if err := s.app.client.Shutdown(); err != nil {
		log.Printf("Shutdown error: %v", err)
		s.app.err = "shutdown: " + err.Error()
	} else {
		s.app.err = "shutting down..."
	}
}
