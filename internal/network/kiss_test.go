package network

import (
	"encoding/hex"
	"testing"
)

func TestKISSEncodeSimpleData(t *testing.T) {
	payload, _ := hex.DecodeString("48656c6c6f")
	frame := &KISSFrame{Port: 0, Command: KISSDataFrame, Data: payload}
	got := hex.EncodeToString(frame.Encode())
	want := "c00048656c6c6fc0"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKISSEscapeFEND(t *testing.T) {
	payload, _ := hex.DecodeString("41c042")
	frame := &KISSFrame{Port: 0, Command: KISSDataFrame, Data: payload}
	got := hex.EncodeToString(frame.Encode())
	want := "c00041dbdc42c0"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKISSEscapeFESC(t *testing.T) {
	payload, _ := hex.DecodeString("41db42")
	frame := &KISSFrame{Port: 0, Command: KISSDataFrame, Data: payload}
	got := hex.EncodeToString(frame.Encode())
	want := "c00041dbdd42c0"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKISSEscapeBoth(t *testing.T) {
	payload, _ := hex.DecodeString("c0db")
	frame := &KISSFrame{Port: 0, Command: KISSDataFrame, Data: payload}
	got := hex.EncodeToString(frame.Encode())
	want := "c000dbdcdbddc0"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKISSPort2(t *testing.T) {
	payload, _ := hex.DecodeString("58")
	frame := &KISSFrame{Port: 2, Command: KISSDataFrame, Data: payload}
	got := hex.EncodeToString(frame.Encode())
	want := "c02058c0"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKISSDecodeSimple(t *testing.T) {
	data, _ := hex.DecodeString("c00048656c6c6fc0")
	frame := DecodeKISSFrame(data)
	if frame == nil {
		t.Fatal("expected frame")
	}
	if frame.Port != 0 {
		t.Errorf("port: got %d, want 0", frame.Port)
	}
	if hex.EncodeToString(frame.Data) != "48656c6c6f" {
		t.Errorf("data: got %s", hex.EncodeToString(frame.Data))
	}
}

func TestKISSDecodeWithEscapes(t *testing.T) {
	data, _ := hex.DecodeString("c000dbdcdbddc0")
	frame := DecodeKISSFrame(data)
	if frame == nil {
		t.Fatal("expected frame")
	}
	if hex.EncodeToString(frame.Data) != "c0db" {
		t.Errorf("data: got %s, want c0db", hex.EncodeToString(frame.Data))
	}
}

func TestKISSInvalidUnknownCommand(t *testing.T) {
	data, _ := hex.DecodeString("c00748656c6c6fc0")
	frame := DecodeKISSFrame(data)
	if frame != nil {
		t.Error("expected nil for unknown command 0x07")
	}
}

func TestKISSInvalidIncompleteEscape(t *testing.T) {
	data, _ := hex.DecodeString("c00041dbc0")
	frame := DecodeKISSFrame(data)
	if frame != nil {
		t.Error("expected nil for incomplete escape")
	}
}

func TestKISSInvalidBadEscapeSequence(t *testing.T) {
	data, _ := hex.DecodeString("c00041db42c0")
	frame := DecodeKISSFrame(data)
	if frame != nil {
		t.Error("expected nil for bad escape byte")
	}
}

func TestKISSProtocolRoundTrip(t *testing.T) {
	kp := NewKISSProtocol(0)
	payload := []byte("Hello RFMP")
	encoded := kp.EncodeData(payload)
	frames := kp.DecodeFrames(encoded)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if string(frames[0].Data) != "Hello RFMP" {
		t.Errorf("data: got %q", string(frames[0].Data))
	}
}

func TestKISSProtocolMultipleFrames(t *testing.T) {
	kp := NewKISSProtocol(0)
	data1 := kp.EncodeData([]byte("one"))
	data2 := kp.EncodeData([]byte("two"))
	combined := append(data1, data2...)
	frames := kp.DecodeFrames(combined)
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if string(frames[0].Data) != "one" || string(frames[1].Data) != "two" {
		t.Errorf("got %q and %q", string(frames[0].Data), string(frames[1].Data))
	}
}

func TestKISSProtocolReset(t *testing.T) {
	kp := NewKISSProtocol(0)
	// Feed partial frame data
	kp.DecodeFrames([]byte{FEND, 0x00, 0x41, 0x42})
	kp.Reset()
	// After reset, new complete frame should decode fine
	encoded := kp.EncodeData([]byte("after-reset"))
	frames := kp.DecodeFrames(encoded)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame after reset, got %d", len(frames))
	}
	if string(frames[0].Data) != "after-reset" {
		t.Errorf("data: %q", string(frames[0].Data))
	}
}

func TestKISSProtocolPartialThenComplete(t *testing.T) {
	kp := NewKISSProtocol(0)
	encoded := kp.EncodeData([]byte("split"))
	// Feed first half
	mid := len(encoded) / 2
	frames := kp.DecodeFrames(encoded[:mid])
	if len(frames) != 0 {
		t.Fatalf("expected 0 frames from partial, got %d", len(frames))
	}
	// Feed second half
	frames = kp.DecodeFrames(encoded[mid:])
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame after complete, got %d", len(frames))
	}
	if string(frames[0].Data) != "split" {
		t.Errorf("data: %q", string(frames[0].Data))
	}
}

func TestKISSDecodeEmpty(t *testing.T) {
	frame := DecodeKISSFrame([]byte{})
	if frame != nil {
		t.Error("expected nil for empty data")
	}
}

func TestKISSDecodeOnlyFENDs(t *testing.T) {
	frame := DecodeKISSFrame([]byte{FEND, FEND})
	if frame != nil {
		t.Error("expected nil for FEND-only frame")
	}
}

func TestKISSProtocolGarbageBeforeFrame(t *testing.T) {
	kp := NewKISSProtocol(0)
	encoded := kp.EncodeData([]byte("valid"))
	// Prepend garbage
	garbage := append([]byte{0x41, 0x42, 0x43}, encoded...)
	frames := kp.DecodeFrames(garbage)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if string(frames[0].Data) != "valid" {
		t.Errorf("data: %q", string(frames[0].Data))
	}
}

func TestKISSDecodeValidCommands(t *testing.T) {
	// TxDelay command (0x01) - should decode but won't be emitted by DecodeFrames
	data := []byte{FEND, 0x01, 0x50, FEND}
	frame := DecodeKISSFrame(data)
	if frame == nil {
		t.Fatal("expected frame for TxDelay command")
	}
	if frame.Command != KISSTxDelay {
		t.Errorf("expected TxDelay, got %d", frame.Command)
	}
}
