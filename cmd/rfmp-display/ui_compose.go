package main

import (
	"fmt"
	"log"
)

type ComposeScreen struct {
	app    *App
	buf    []rune
	cursor int
}

func NewComposeScreen(app *App, initial rune) *ComposeScreen {
	s := &ComposeScreen{app: app}
	if initial != 0 {
		s.buf = []rune{initial}
		s.cursor = 1
	}
	return s
}

func (s *ComposeScreen) OnEnter() {}

func (s *ComposeScreen) Render(fb *Framebuffer) {
	y := StatusBarH + 4
	fb.DrawString(4, y, "Compose", ColorBlue, ColorBlack)
	y += FontH + 4

	fb.HLine(0, y, ScreenW, ColorDarkGray)
	y += 4

	text := string(s.buf)
	if s.cursor < len(s.buf) {
		text = string(s.buf[:s.cursor]) + "_" + string(s.buf[s.cursor:])
	} else {
		text = string(s.buf) + "_"
	}

	fb.DrawStringWrap(4, y, ScreenW-8, text, ColorWhite, ColorBlack)

	counter := fmt.Sprintf("%d", len(s.buf))
	cx := ScreenW - (len(counter)+1)*FontW
	fb.DrawString(cx, ScreenH-FontH, counter, ColorLightGray, ColorBlack)

	fb.DrawString(4, ScreenH-FontH, "Enter:send Esc:cancel", ColorLightGray, ColorBlack)
}

func (s *ComposeScreen) HandleKey(ev KeyEvent) Screen {
	switch ev.Code {
	case KeyEnter:
		if len(s.buf) > 0 {
			s.send()
		}
		return NewTimelineScreen(s.app)
	case KeyBack:
		return NewTimelineScreen(s.app)
	case KeyBackspace:
		if s.cursor > 0 {
			s.buf = append(s.buf[:s.cursor-1], s.buf[s.cursor:]...)
			s.cursor--
		}
	case KeyLeft:
		if s.cursor > 0 {
			s.cursor--
		}
	case KeyRight:
		if s.cursor < len(s.buf) {
			s.cursor++
		}
	default:
		if ev.Key != 0 {
			s.buf = append(s.buf[:s.cursor], append([]rune{ev.Key}, s.buf[s.cursor:]...)...)
			s.cursor++
		}
	}
	return s
}

func (s *ComposeScreen) send() {
	body := string(s.buf)
	var author *string
	if s.app.author != "" {
		author = &s.app.author
	}
	if _, err := s.app.client.SendMessage(s.app.channel, body, author, nil); err != nil {
		log.Printf("send failed: %v", err)
	}
}
