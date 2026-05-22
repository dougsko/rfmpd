package sync

import (
	"math/rand"
	"sync"
	"time"
)

type Timing struct {
	baseDelay float64
	jitter    float64

	mu             sync.Mutex
	transmissions  int
	lastTransmit   time.Time
}

func NewTiming(baseDelay, jitter float64) *Timing {
	return &Timing{
		baseDelay: baseDelay,
		jitter:    jitter,
	}
}

func (t *Timing) CalculateDelay() time.Duration {
	delay := t.baseDelay + rand.Float64()*t.jitter
	return time.Duration(delay * float64(time.Second))
}

func (t *Timing) CalculateSyncDelay() time.Duration {
	return t.CalculateDelay() + time.Duration(rand.Float64()*2.0*float64(time.Second))
}

func (t *Timing) CalculateFragmentDelay(fragmentIndex, total int) time.Duration {
	if fragmentIndex == 0 {
		return t.CalculateDelay()
	}
	delay := 0.05 + rand.Float64()*0.05
	return time.Duration(delay * float64(time.Second))
}

func (t *Timing) CalculateRebroadcastDelay() time.Duration {
	base := t.CalculateDelay()
	extra := 1.0 + rand.Float64()*2.0
	return base + time.Duration(extra*float64(time.Second))
}

func (t *Timing) RecordTransmission() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.transmissions++
	t.lastTransmit = time.Now()
}

func (t *Timing) GetStats() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	stats := map[string]interface{}{
		"base_delay":    t.baseDelay,
		"jitter":        t.jitter,
		"transmissions": t.transmissions,
	}
	if !t.lastTransmit.IsZero() {
		stats["last_transmit"] = t.lastTransmit.Format(time.RFC3339)
	}
	return stats
}
