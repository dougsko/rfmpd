package main

type Framebuffer struct {
	Pix  []uint16
	W, H int
}

func NewFramebuffer(w, h int) *Framebuffer {
	return &Framebuffer{
		Pix: make([]uint16, w*h),
		W:   w,
		H:   h,
	}
}

func (fb *Framebuffer) SetPixel(x, y int, c uint16) {
	if x >= 0 && x < fb.W && y >= 0 && y < fb.H {
		fb.Pix[y*fb.W+x] = c
	}
}

func (fb *Framebuffer) Clear(c uint16) {
	for i := range fb.Pix {
		fb.Pix[i] = c
	}
}

func (fb *Framebuffer) FillRect(x, y, w, h int, c uint16) {
	for row := y; row < y+h; row++ {
		if row < 0 || row >= fb.H {
			continue
		}
		for col := x; col < x+w; col++ {
			if col >= 0 && col < fb.W {
				fb.Pix[row*fb.W+col] = c
			}
		}
	}
}

func (fb *Framebuffer) HLine(x, y, w int, c uint16) {
	if y < 0 || y >= fb.H {
		return
	}
	for col := x; col < x+w; col++ {
		if col >= 0 && col < fb.W {
			fb.Pix[y*fb.W+col] = c
		}
	}
}

func (fb *Framebuffer) DrawChar(x, y int, ch byte, fg, bg uint16) {
	glyph := font8x16[ch]
	for row := 0; row < 16; row++ {
		py := y + row
		if py < 0 || py >= fb.H {
			continue
		}
		bits := glyph[row]
		for col := 0; col < 8; col++ {
			px := x + col
			if px < 0 || px >= fb.W {
				continue
			}
			if bits&(0x80>>col) != 0 {
				fb.Pix[py*fb.W+px] = fg
			} else {
				fb.Pix[py*fb.W+px] = bg
			}
		}
	}
}

func (fb *Framebuffer) DrawString(x, y int, s string, fg, bg uint16) {
	cx := x
	for i := 0; i < len(s); i++ {
		if cx+8 > fb.W {
			break
		}
		fb.DrawChar(cx, y, s[i], fg, bg)
		cx += 8
	}
}

func (fb *Framebuffer) DrawStringWrap(x, y, maxW int, s string, fg, bg uint16) int {
	cx := x
	cy := y
	lines := 1
	cols := maxW / 8

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || (cx-x)/8 >= cols {
			cx = x
			cy += 16
			lines++
			if s[i] == '\n' {
				continue
			}
		}
		if cy+16 > fb.H {
			break
		}
		fb.DrawChar(cx, cy, s[i], fg, bg)
		cx += 8
	}
	return lines
}
