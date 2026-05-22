package main

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

const FEND = 0xC0

type RFCondition struct {
	DropRate    float64 // probability of dropping a frame entirely (0.0-1.0)
	CorruptRate float64 // probability of bit corruption in a delivered frame (0.0-1.0)
	BurstFade   bool    // enable burst fading (periodic signal loss)
	FadeCycleMs int     // burst fade: total cycle length in ms
	FadeDownMs  int     // burst fade: how long the signal is down per cycle
}

type Broker struct {
	port     int
	listener net.Listener
	verbose  bool

	mu      sync.RWMutex
	clients map[int]net.Conn
	nextID  int

	partitionA map[int]bool
	partitionB map[int]bool
	dropRate   float64
	baudRate   int // 0 = unlimited, otherwise bits per second
	txMu       sync.Mutex // serializes transmissions to simulate half-duplex channel

	rfCondition *RFCondition
	startTime   time.Time
}

func NewBroker(port int, verbose bool) *Broker {
	return &Broker{
		port:      port,
		verbose:   verbose,
		clients:   make(map[int]net.Conn),
		startTime: time.Now(),
	}
}

func (b *Broker) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", b.port))
	if err != nil {
		return err
	}
	b.listener = ln
	go b.acceptLoop()
	return nil
}

func (b *Broker) Stop() {
	if b.listener != nil {
		b.listener.Close()
	}
	b.mu.Lock()
	for _, conn := range b.clients {
		conn.Close()
	}
	b.clients = make(map[int]net.Conn)
	b.mu.Unlock()
}

func (b *Broker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, conn := range b.clients {
		conn.Close()
	}
	b.clients = make(map[int]net.Conn)
	b.nextID = 0
	b.partitionA = nil
	b.partitionB = nil
	b.dropRate = 0
	b.rfCondition = nil
	b.startTime = time.Now()
}

func (b *Broker) Partition(groupA, groupB []int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.partitionA = make(map[int]bool)
	b.partitionB = make(map[int]bool)
	for _, id := range groupA {
		b.partitionA[id] = true
	}
	for _, id := range groupB {
		b.partitionB[id] = true
	}
}

func (b *Broker) Heal() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.partitionA = nil
	b.partitionB = nil
}

func (b *Broker) SetDropRate(pct float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dropRate = pct
}

func (b *Broker) SetBaudRate(baud int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.baudRate = baud
}

func (b *Broker) SetRFCondition(cond *RFCondition) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rfCondition = cond
	if cond != nil && cond.DropRate > 0 {
		b.dropRate = cond.DropRate
	} else if cond == nil {
		b.dropRate = 0
	}
}

func (b *Broker) acceptLoop() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		id := b.nextID
		b.nextID++
		b.clients[id] = conn
		b.mu.Unlock()

		if b.verbose {
			fmt.Printf("[broker] client %d connected from %s\n", id, conn.RemoteAddr())
		}
		go b.readLoop(id, conn)
	}
}

func (b *Broker) readLoop(id int, conn net.Conn) {
	buf := make([]byte, 4096)
	var frameBuf []byte

	for {
		n, err := conn.Read(buf)
		if err != nil {
			b.mu.Lock()
			delete(b.clients, id)
			b.mu.Unlock()
			if b.verbose {
				fmt.Printf("[broker] client %d disconnected\n", id)
			}
			return
		}

		frameBuf = append(frameBuf, buf[:n]...)
		frames := splitKISSFrames(&frameBuf)
		for _, frame := range frames {
			b.broadcast(id, frame)
		}
	}
}

func (b *Broker) broadcast(senderID int, frame []byte) {
	b.mu.RLock()
	baud := b.baudRate
	cond := b.rfCondition
	targets := make(map[int]net.Conn)
	for targetID, conn := range b.clients {
		if targetID == senderID {
			continue
		}
		if !b.canDeliver(senderID, targetID) {
			continue
		}
		if b.dropRate > 0 && rand.Float64() < b.dropRate {
			if b.verbose {
				fmt.Printf("[broker] dropped frame %d -> %d (random loss)\n", senderID, targetID)
			}
			continue
		}
		targets[targetID] = conn
	}
	b.mu.RUnlock()

	// Burst fading: if signal is in a fade trough, drop the entire frame
	if cond != nil && cond.BurstFade && cond.FadeCycleMs > 0 {
		elapsed := time.Since(b.startTime).Milliseconds()
		posInCycle := elapsed % int64(cond.FadeCycleMs)
		if posInCycle < int64(cond.FadeDownMs) {
			if b.verbose {
				fmt.Printf("[broker] dropped frame from %d (burst fade)\n", senderID)
			}
			return
		}
	}

	// Simulate half-duplex channel: serialize transmissions and apply baud delay
	if baud > 0 {
		b.txMu.Lock()
		bits := len(frame) * 8
		txTime := time.Duration(float64(bits) / float64(baud) * float64(time.Second))
		time.Sleep(txTime)
		b.txMu.Unlock()
	}

	for targetID, conn := range targets {
		outFrame := frame
		// Bit corruption: flip random bits to simulate noise
		if cond != nil && cond.CorruptRate > 0 && rand.Float64() < cond.CorruptRate {
			outFrame = corruptFrame(frame)
			if b.verbose {
				fmt.Printf("[broker] corrupted frame %d -> %d\n", senderID, targetID)
			}
		}
		conn.Write(outFrame)
	}

	if b.verbose {
		fmt.Printf("[broker] relayed frame from %d (%d bytes)\n", senderID, len(frame))
	}
}

func corruptFrame(frame []byte) []byte {
	corrupted := make([]byte, len(frame))
	copy(corrupted, frame)
	// Don't corrupt the FEND delimiters (first and last bytes) — the KISS layer
	// needs them intact to frame the data. Corrupt 1-3 random bytes in the payload.
	if len(corrupted) <= 2 {
		return corrupted
	}
	numFlips := 1 + rand.Intn(3)
	for i := 0; i < numFlips; i++ {
		idx := 1 + rand.Intn(len(corrupted)-2)
		bit := byte(1 << rand.Intn(8))
		corrupted[idx] ^= bit
	}
	return corrupted
}

func (b *Broker) canDeliver(from, to int) bool {
	if b.partitionA == nil {
		return true
	}
	fromInA := b.partitionA[from]
	toInA := b.partitionA[to]
	fromInB := b.partitionB[from]
	toInB := b.partitionB[to]

	if fromInA && toInB {
		return false
	}
	if fromInB && toInA {
		return false
	}
	return true
}

func splitKISSFrames(buf *[]byte) [][]byte {
	var frames [][]byte
	data := *buf

	for {
		start := -1
		for i, b := range data {
			if b == FEND {
				start = i
				break
			}
		}
		if start == -1 {
			*buf = data
			return frames
		}

		end := -1
		for i := start + 1; i < len(data); i++ {
			if data[i] == FEND {
				end = i
				break
			}
		}
		if end == -1 {
			*buf = data[start:]
			return frames
		}

		frame := make([]byte, end-start+1)
		copy(frame, data[start:end+1])
		frames = append(frames, frame)
		data = data[end+1:]
	}
}
