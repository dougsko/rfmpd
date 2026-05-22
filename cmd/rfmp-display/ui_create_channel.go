package main

import "log"

type CreateChannelScreen struct {
	app    *App
	buf    []rune
	cursor int
}

func NewCreateChannelScreen(app *App) *CreateChannelScreen {
	return &CreateChannelScreen{app: app}
}

func (s *CreateChannelScreen) OnEnter() {}

func (s *CreateChannelScreen) Render(fb *Framebuffer) {
	y := StatusBarH + 4
	fb.DrawString(4, y, "New Channel", ColorBlue, ColorBlack)
	y += FontH + 4

	fb.HLine(0, y, ScreenW, ColorDarkGray)
	y += 8

	fb.DrawString(4, y, "Name:", ColorLightGray, ColorBlack)
	y += FontH + 4

	text := string(s.buf) + "_"
	fb.DrawString(4, y, text, ColorWhite, ColorBlack)

	fb.DrawString(4, ScreenH-FontH, "Enter:create Esc:cancel", ColorLightGray, ColorBlack)
}

func (s *CreateChannelScreen) HandleKey(ev KeyEvent) Screen {
	switch ev.Code {
	case KeyEnter:
		if len(s.buf) > 0 {
			name := string(s.buf)
			if err := s.app.client.CreateChannel(name); err != nil {
				log.Printf("CreateChannel error: %v", err)
				s.app.err = "create: " + err.Error()
			} else {
				s.app.err = ""
			}
		}
		return NewChannelListScreen(s.app)
	case KeyBack:
		return NewChannelListScreen(s.app)
	case KeyBackspace:
		if s.cursor > 0 {
			s.buf = append(s.buf[:s.cursor-1], s.buf[s.cursor:]...)
			s.cursor--
		}
	default:
		if ev.Key != 0 && ev.Key != ' ' {
			r := ev.Key
			if r >= 'A' && r <= 'Z' {
				r = r + 32
			}
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				s.buf = append(s.buf[:s.cursor], append([]rune{r}, s.buf[s.cursor:]...)...)
				s.cursor++
			}
		}
	}
	return s
}
