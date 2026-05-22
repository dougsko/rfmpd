//go:build hw

package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

const (
	spiDevice = "SPI0.0"
	spiHz     = 40 * physic.MegaHertz

	// Orange Pi Zero 2W GPIO sysfs numbers (Allwinner H618)
	// Formula: bank * 32 + pin (PA=0, PB=32, ... PH=224, PI=256)
	gpioDC  = "228" // PH4 - Display Data/Command (physical pin 18)
	gpioRST = "257" // PI1 - Display Reset (physical pin 12)
	gpioINT = "269" // PI13 - Keyboard INT active low (physical pin 7)
	gpioLED = "256" // PI0 - Status LED (physical pin 29)

	kbI2CBus  = "1"
	kbI2CAddr = 0x1F

	regKeyFIFO   = 0x09
	regBacklight = 0x05

	bbqUp        = 0x01
	bbqDown      = 0x02
	bbqLeft      = 0x03
	bbqRight     = 0x04
	bbqCenter    = 0x05
	bbqBack      = 0x06
	bbqEnter     = 0x07
	bbqBackspace = 0x08

	kbStatePressed   = 1
	kbStateLongPress = 2
)

var (
	hostInitOnce sync.Once
	hostInitErr  error
)

func initHost() error {
	hostInitOnce.Do(func() { _, hostInitErr = host.Init() })
	return hostInitErr
}

// --- Display ---

type HWDisplay struct {
	port   spi.PortCloser
	conn   spi.Conn
	dc     gpio.PinOut
	rst    gpio.PinOut
	spiBuf [ScreenW * ScreenH * 2]byte
}

func newDisplay() Display { return &HWDisplay{} }

func (d *HWDisplay) Init() error {
	if err := initHost(); err != nil {
		return fmt.Errorf("periph init: %w", err)
	}

	d.dc = gpioreg.ByName(gpioDC)
	if d.dc == nil {
		return fmt.Errorf("gpio %s (DC) not found", gpioDC)
	}
	d.rst = gpioreg.ByName(gpioRST)
	if d.rst == nil {
		return fmt.Errorf("gpio %s (RST) not found", gpioRST)
	}

	port, err := spireg.Open(spiDevice)
	if err != nil {
		return fmt.Errorf("spi open: %w", err)
	}
	d.port = port

	conn, err := d.port.Connect(spiHz, spi.Mode0, 8)
	if err != nil {
		d.port.Close()
		return fmt.Errorf("spi connect: %w", err)
	}
	d.conn = conn

	d.hardwareReset()
	return d.initSequence()
}

func (d *HWDisplay) hardwareReset() {
	d.rst.Out(gpio.Low)
	time.Sleep(10 * time.Millisecond)
	d.rst.Out(gpio.High)
	time.Sleep(120 * time.Millisecond)
}

type initWriter struct {
	d   *HWDisplay
	err error
}

func (w *initWriter) cmd(c byte) {
	if w.err != nil {
		return
	}
	w.err = w.d.writeCmd(c)
}

func (w *initWriter) data(b ...byte) {
	if w.err != nil {
		return
	}
	w.err = w.d.writeData(b...)
}

func (w *initWriter) sleep(dur time.Duration) {
	if w.err != nil {
		return
	}
	time.Sleep(dur)
}

func (d *HWDisplay) writeCmd(cmd byte) error {
	d.dc.Out(gpio.Low)
	return d.conn.Tx([]byte{cmd}, nil)
}

func (d *HWDisplay) writeData(data ...byte) error {
	d.dc.Out(gpio.High)
	return d.conn.Tx(data, nil)
}

func (d *HWDisplay) initSequence() error {
	w := &initWriter{d: d}

	// Software reset
	w.cmd(0x01)
	w.sleep(5 * time.Millisecond)

	// Display off
	w.cmd(0x28)

	// Power control A
	w.cmd(0xCB)
	w.data(0x39, 0x2C, 0x00, 0x34, 0x02)

	// Power control B
	w.cmd(0xCF)
	w.data(0x00, 0xC1, 0x30)

	// Driver timing control A
	w.cmd(0xE8)
	w.data(0x85, 0x00, 0x78)

	// Driver timing control B
	w.cmd(0xEA)
	w.data(0x00, 0x00)

	// Power on sequence control
	w.cmd(0xED)
	w.data(0x64, 0x03, 0x12, 0x81)

	// Pump ratio control
	w.cmd(0xF7)
	w.data(0x20)

	// Power control 1
	w.cmd(0xC0)
	w.data(0x23)

	// Power control 2
	w.cmd(0xC1)
	w.data(0x10)

	// VCOM control 1
	w.cmd(0xC5)
	w.data(0x3E, 0x28)

	// VCOM control 2
	w.cmd(0xC7)
	w.data(0x86)

	// Memory access control (MADCTL) - landscape rotation (MX+MV)
	w.cmd(0x36)
	w.data(0x28)

	// Pixel format - 16 bit
	w.cmd(0x3A)
	w.data(0x55)

	// Frame rate control
	w.cmd(0xB1)
	w.data(0x00, 0x18)

	// Display function control
	w.cmd(0xB6)
	w.data(0x08, 0x82, 0x27)

	// 3Gamma function disable
	w.cmd(0xF2)
	w.data(0x00)

	// Gamma curve selected
	w.cmd(0x26)
	w.data(0x01)

	// Positive gamma correction
	w.cmd(0xE0)
	w.data(0x0F, 0x31, 0x2B, 0x0C, 0x0E, 0x08, 0x4E, 0xF1,
		0x37, 0x07, 0x10, 0x03, 0x0E, 0x09, 0x00)

	// Negative gamma correction
	w.cmd(0xE1)
	w.data(0x00, 0x0E, 0x14, 0x03, 0x11, 0x07, 0x31, 0xC1,
		0x48, 0x08, 0x0F, 0x0C, 0x31, 0x36, 0x0F)

	// Sleep out
	w.cmd(0x11)
	w.sleep(120 * time.Millisecond)

	// Display on
	w.cmd(0x29)
	w.sleep(20 * time.Millisecond)

	// Set column address (0-319)
	w.cmd(0x2A)
	w.data(0x00, 0x00, 0x01, 0x3F)

	// Set row address (0-239)
	w.cmd(0x2B)
	w.data(0x00, 0x00, 0x00, 0xEF)

	return w.err
}

func (d *HWDisplay) Flush(pix []uint16, w, h int) error {
	for i, p := range pix {
		d.spiBuf[i*2] = byte(p >> 8)
		d.spiBuf[i*2+1] = byte(p)
	}
	if err := d.writeCmd(0x2C); err != nil {
		return err
	}
	d.dc.Out(gpio.High)
	return d.conn.Tx(d.spiBuf[:], nil)
}

func (d *HWDisplay) Close() {
	if d.port != nil {
		d.port.Close()
	}
}

// --- Keyboard ---

type HWKeyboard struct {
	bus    i2c.BusCloser
	dev    *i2c.Dev
	intPin gpio.PinIn
	events chan KeyEvent
	done   chan struct{}
}

func newKeyboard() Keyboard {
	return &HWKeyboard{
		events: make(chan KeyEvent, 32),
		done:   make(chan struct{}),
	}
}

func (kb *HWKeyboard) Init() error {
	if err := initHost(); err != nil {
		return fmt.Errorf("periph init: %w", err)
	}

	bus, err := i2creg.Open(kbI2CBus)
	if err != nil {
		return fmt.Errorf("i2c open: %w", err)
	}
	kb.bus = bus
	kb.dev = &i2c.Dev{Bus: kb.bus, Addr: kbI2CAddr}

	// Set keyboard backlight to ~25%
	if err := kb.dev.Tx([]byte{regBacklight, 0x40}, nil); err != nil {
		kb.bus.Close()
		return fmt.Errorf("keyboard backlight: %w", err)
	}

	pin := gpioreg.ByName(gpioINT)
	if pin == nil {
		kb.bus.Close()
		return fmt.Errorf("gpio %s (INT) not found", gpioINT)
	}
	kb.intPin = pin.(gpio.PinIn)
	if err := kb.intPin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		kb.bus.Close()
		return fmt.Errorf("gpio edge setup: %w", err)
	}

	kb.drainFIFO()
	go kb.readLoop()
	return nil
}

func (kb *HWKeyboard) Events() <-chan KeyEvent { return kb.events }

func (kb *HWKeyboard) Poll() bool { return true }

func (kb *HWKeyboard) Close() {
	close(kb.done)
	if kb.intPin != nil {
		kb.intPin.In(gpio.PullUp, gpio.NoEdge)
	}
	if kb.bus != nil {
		kb.bus.Close()
	}
}

func (kb *HWKeyboard) readLoop() {
	for {
		if !kb.intPin.WaitForEdge(0) {
			return
		}
		select {
		case <-kb.done:
			return
		default:
		}
		kb.drainFIFO()
	}
}

func (kb *HWKeyboard) drainFIFO() {
	buf := make([]byte, 2)
	for {
		if err := kb.dev.Tx([]byte{regKeyFIFO}, buf); err != nil {
			log.Printf("keyboard: i2c read error: %v", err)
			return
		}
		state, keycode := buf[0], buf[1]
		if state == 0 && keycode == 0 {
			return
		}
		if ev, ok := kb.mapKey(state, keycode); ok {
			select {
			case kb.events <- ev:
			default:
			}
		}
	}
}

func (kb *HWKeyboard) mapKey(state, keycode byte) (KeyEvent, bool) {
	if state != kbStatePressed && state != kbStateLongPress {
		return KeyEvent{}, false
	}
	switch keycode {
	case bbqUp:
		return KeyEvent{Code: KeyUp}, true
	case bbqDown:
		return KeyEvent{Code: KeyDown}, true
	case bbqLeft:
		return KeyEvent{Code: KeyLeft}, true
	case bbqRight:
		return KeyEvent{Code: KeyRight}, true
	case bbqCenter:
		return KeyEvent{Code: KeyEnter}, true
	case bbqBack:
		return KeyEvent{Code: KeyBack}, true
	case bbqEnter:
		return KeyEvent{Code: KeyEnter}, true
	case bbqBackspace:
		return KeyEvent{Code: KeyBackspace}, true
	default:
		if keycode >= 0x20 && keycode <= 0x7E {
			return KeyEvent{Key: rune(keycode)}, true
		}
	}
	return KeyEvent{}, false
}

// --- LED ---

type HWLED struct {
	pin       gpio.PinOut
	mu        sync.Mutex
	on        bool
	blinkStop chan struct{}
}

func newLED() LED { return &HWLED{} }

func (l *HWLED) Init() error {
	if err := initHost(); err != nil {
		return fmt.Errorf("periph init: %w", err)
	}

	pin := gpioreg.ByName(gpioLED)
	if pin == nil {
		return fmt.Errorf("gpio %s (LED) not found", gpioLED)
	}
	l.pin = pin.(gpio.PinOut)
	return l.pin.Out(gpio.Low)
}

func (l *HWLED) On() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.on = true
	l.pin.Out(gpio.High)
}

func (l *HWLED) Off() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.on = false
	l.pin.Out(gpio.Low)
}

func (l *HWLED) IsOn() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.on
}

func (l *HWLED) Blink(n int, interval time.Duration) {
	l.mu.Lock()
	if l.blinkStop != nil {
		close(l.blinkStop)
	}
	stop := make(chan struct{})
	l.blinkStop = stop
	l.mu.Unlock()

	go func() {
		for i := 0; i < n; i++ {
			select {
			case <-stop:
				l.Off()
				return
			default:
			}
			l.On()
			select {
			case <-stop:
				l.Off()
				return
			case <-time.After(interval):
			}
			l.Off()
			select {
			case <-stop:
				return
			case <-time.After(interval):
			}
		}
	}()
}

func (l *HWLED) Close() {
	l.mu.Lock()
	if l.blinkStop != nil {
		close(l.blinkStop)
		l.blinkStop = nil
	}
	l.mu.Unlock()
	l.Off()
}
