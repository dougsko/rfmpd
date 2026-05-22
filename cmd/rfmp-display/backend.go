package main

import "time"

const (
	KeyEnter     = 0x100
	KeyBack      = 0x101
	KeyBackspace = 0x102
	KeyUp        = 0x103
	KeyDown      = 0x104
	KeyLeft      = 0x105
	KeyRight     = 0x106
	KeySym       = 0x107
)

type KeyEvent struct {
	Key  rune
	Code int
}

type Display interface {
	Init() error
	Flush(pix []uint16, w, h int) error
	Close()
}

type Keyboard interface {
	Init() error
	Events() <-chan KeyEvent
	Poll() bool
	Close()
}

type LED interface {
	Init() error
	On()
	Off()
	IsOn() bool
	Blink(n int, interval time.Duration)
	Close()
}
