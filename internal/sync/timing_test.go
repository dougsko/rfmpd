package sync

import (
	"testing"
	"time"
)

func TestCalculateDelay(t *testing.T) {
	tm := NewTiming(0.2, 0.4)
	min := 200 * time.Millisecond
	max := 600 * time.Millisecond

	for i := 0; i < 100; i++ {
		d := tm.CalculateDelay()
		if d < min || d > max {
			t.Fatalf("CalculateDelay() = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestCalculateSyncDelay(t *testing.T) {
	tm := NewTiming(0.2, 0.4)
	min := 200 * time.Millisecond
	max := 2600 * time.Millisecond // base+jitter+2s

	for i := 0; i < 100; i++ {
		d := tm.CalculateSyncDelay()
		if d < min || d > max {
			t.Fatalf("CalculateSyncDelay() = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestCalculateFragmentDelay_FirstFragment(t *testing.T) {
	tm := NewTiming(0.2, 0.4)
	min := 200 * time.Millisecond
	max := 600 * time.Millisecond

	for i := 0; i < 100; i++ {
		d := tm.CalculateFragmentDelay(0, 5)
		if d < min || d > max {
			t.Fatalf("CalculateFragmentDelay(0, 5) = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestCalculateFragmentDelay_SubsequentFragments(t *testing.T) {
	tm := NewTiming(0.2, 0.4)
	min := 50 * time.Millisecond
	max := 100 * time.Millisecond

	for i := 0; i < 100; i++ {
		d := tm.CalculateFragmentDelay(2, 5)
		if d < min || d > max {
			t.Fatalf("CalculateFragmentDelay(2, 5) = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestCalculateRebroadcastDelay(t *testing.T) {
	tm := NewTiming(0.2, 0.4)
	min := 1200 * time.Millisecond  // base + 1s minimum extra
	max := 3600 * time.Millisecond  // base+jitter + 3s max extra

	for i := 0; i < 100; i++ {
		d := tm.CalculateRebroadcastDelay()
		if d < min || d > max {
			t.Fatalf("CalculateRebroadcastDelay() = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestRecordTransmission(t *testing.T) {
	tm := NewTiming(0.1, 0.1)

	stats := tm.GetStats()
	if stats["transmissions"].(int) != 0 {
		t.Fatalf("expected 0 transmissions initially")
	}
	if _, ok := stats["last_transmit"]; ok {
		t.Fatalf("expected no last_transmit initially")
	}

	tm.RecordTransmission()
	tm.RecordTransmission()

	stats = tm.GetStats()
	if stats["transmissions"].(int) != 2 {
		t.Fatalf("expected 2 transmissions, got %v", stats["transmissions"])
	}
	if _, ok := stats["last_transmit"]; !ok {
		t.Fatalf("expected last_transmit after RecordTransmission")
	}
}

func TestGetStats(t *testing.T) {
	tm := NewTiming(0.5, 1.0)
	stats := tm.GetStats()

	if stats["base_delay"].(float64) != 0.5 {
		t.Fatalf("expected base_delay 0.5, got %v", stats["base_delay"])
	}
	if stats["jitter"].(float64) != 1.0 {
		t.Fatalf("expected jitter 1.0, got %v", stats["jitter"])
	}
}
