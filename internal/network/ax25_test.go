package network

import (
	"encoding/hex"
	"testing"
)

func TestAX25EncodeBasicLast(t *testing.T) {
	addr, _ := NewAX25Address("N0CALL", 0)
	got := hex.EncodeToString(addr.Encode(true))
	want := "9c608682989861"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAX25EncodeWithSSIDNotLast(t *testing.T) {
	addr, _ := NewAX25Address("N0CALL", 1)
	got := hex.EncodeToString(addr.Encode(false))
	want := "9c608682989862"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAX25EncodeShortCallsignPadded(t *testing.T) {
	addr, _ := NewAX25Address("RFMP", 0)
	got := hex.EncodeToString(addr.Encode(false))
	want := "a48c9aa0404060"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAX25EncodeMaxSSID(t *testing.T) {
	addr, _ := NewAX25Address("W1AW", 15)
	got := hex.EncodeToString(addr.Encode(true))
	want := "ae6282ae40407f"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAX25DecodeAddress(t *testing.T) {
	data, _ := hex.DecodeString("9c608682989861")
	addr := DecodeAX25Address(data)
	if addr == nil {
		t.Fatal("expected address")
	}
	if addr.Callsign != "N0CALL" {
		t.Errorf("callsign: %s", addr.Callsign)
	}
	if addr.SSID != 0 {
		t.Errorf("ssid: %d", addr.SSID)
	}
}

func TestAX25DecodeAddressWithSSID(t *testing.T) {
	data, _ := hex.DecodeString("9c608682989862")
	addr := DecodeAX25Address(data)
	if addr == nil {
		t.Fatal("expected address")
	}
	if addr.Callsign != "N0CALL" || addr.SSID != 1 {
		t.Errorf("got %s-%d", addr.Callsign, addr.SSID)
	}
}

func TestAX25UIFrame(t *testing.T) {
	frame, err := CreateUIFrame("N0CALL", "RFMP", []byte("test payload"))
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(frame.Encode())
	want := "a48c9aa04040609c60868298986103f074657374207061796c6f6164"
	if got != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", got, want)
	}
}

func TestAX25UIFrameLength(t *testing.T) {
	frame, _ := CreateUIFrame("N0CALL", "RFMP", []byte("test payload"))
	encoded := frame.Encode()
	if len(encoded) != 28 {
		t.Errorf("length: got %d, want 28", len(encoded))
	}
}

func TestAX25DecodeFrame(t *testing.T) {
	data, _ := hex.DecodeString("a48c9aa04040609c60868298986103f074657374207061796c6f6164")
	frame := DecodeAX25Frame(data)
	if frame == nil {
		t.Fatal("expected frame")
	}
	if frame.Destination.Callsign != "RFMP" {
		t.Errorf("dest: %s", frame.Destination.Callsign)
	}
	if frame.Source.Callsign != "N0CALL" {
		t.Errorf("src: %s", frame.Source.Callsign)
	}
	if frame.Control != 0x03 {
		t.Errorf("control: %02x", frame.Control)
	}
	if frame.PID != 0xF0 {
		t.Errorf("pid: %02x", frame.PID)
	}
	if string(frame.Info) != "test payload" {
		t.Errorf("info: %q", string(frame.Info))
	}
}

func TestAX25ParseAddress(t *testing.T) {
	cases := []struct {
		input    string
		callsign string
		ssid     int
	}{
		{"N0CALL", "N0CALL", 0},
		{"N0CALL-1", "N0CALL", 1},
		{"W1AW-15", "W1AW", 15},
	}
	for _, tc := range cases {
		addr, err := ParseAX25Address(tc.input)
		if err != nil {
			t.Errorf("ParseAX25Address(%q): %v", tc.input, err)
			continue
		}
		if addr.Callsign != tc.callsign || addr.SSID != tc.ssid {
			t.Errorf("ParseAX25Address(%q): got %s-%d", tc.input, addr.Callsign, addr.SSID)
		}
	}
}

func TestFullStackPayloadToKISS(t *testing.T) {
	// Exercises the full AX.25 → KISS framing stack with an arbitrary payload.
	rfmpBytes := []byte("MSG|id=d4728395545d|from=N0CALL|time=20240115T120000Z|chan=general|prio=1|reply=-|body=Hello world")

	ax25Frame, err := CreateUIFrame("N0CALL", "RFMP", rfmpBytes)
	if err != nil {
		t.Fatal(err)
	}
	ax25Encoded := ax25Frame.Encode()

	kissFrame := &KISSFrame{Port: 0, Command: KISSDataFrame, Data: ax25Encoded}
	kissEncoded := kissFrame.Encode()

	wantKISS := "c000a48c9aa04040609c60868298986103f04d53477c69643d6434373238333935353435647c66726f6d3d4e3043414c4c7c74696d653d3230323430313135543132303030305a7c6368616e3d67656e6572616c7c7072696f3d317c7265706c793d2d7c626f64793d48656c6c6f20776f726c64c0"
	gotKISS := hex.EncodeToString(kissEncoded)
	if gotKISS != wantKISS {
		t.Errorf("full stack KISS mismatch:\ngot:\n  %s\nwant:\n  %s", gotKISS, wantKISS)
	}
}

func TestAX25AddressString_NoSSID(t *testing.T) {
	addr := &AX25Address{Callsign: "N0CALL", SSID: 0}
	if addr.String() != "N0CALL" {
		t.Errorf("expected N0CALL, got %s", addr.String())
	}
}

func TestAX25AddressString_WithSSID(t *testing.T) {
	addr := &AX25Address{Callsign: "W1AW", SSID: 5}
	if addr.String() != "W1AW-5" {
		t.Errorf("expected W1AW-5, got %s", addr.String())
	}
}

func TestNewAX25Address_CallsignTooLong(t *testing.T) {
	_, err := NewAX25Address("TOOLONG7", 0)
	if err == nil {
		t.Error("expected error for callsign > 6 chars")
	}
}

func TestNewAX25Address_SSIDOutOfRange(t *testing.T) {
	_, err := NewAX25Address("W1AW", 16)
	if err == nil {
		t.Error("expected error for SSID > 15")
	}
	_, err = NewAX25Address("W1AW", -1)
	if err == nil {
		t.Error("expected error for SSID < 0")
	}
}

func TestParseAX25Address_InvalidSSID(t *testing.T) {
	_, err := ParseAX25Address("W1AW-abc")
	if err == nil {
		t.Error("expected error for non-numeric SSID")
	}
}

func TestParseAX25Address_SSIDOutOfRange(t *testing.T) {
	_, err := ParseAX25Address("W1AW-16")
	if err == nil {
		t.Error("expected error for SSID 16")
	}
}

func TestDecodeAX25Address_TooShort(t *testing.T) {
	addr := DecodeAX25Address([]byte{0x01, 0x02, 0x03})
	if addr != nil {
		t.Error("expected nil for short data")
	}
}

func TestDecodeAX25Frame_TooShort(t *testing.T) {
	frame := DecodeAX25Frame([]byte{0x01, 0x02, 0x03})
	if frame != nil {
		t.Error("expected nil for data < 16 bytes")
	}
}

func TestDecodeAX25Frame_NoControlByte(t *testing.T) {
	// 14 bytes of addresses but nothing after
	data, _ := hex.DecodeString("a48c9aa04040609c6086829898610000")
	// Truncate to only have addresses + 1 byte (control) but no PID
	frame := DecodeAX25Frame(data[:15])
	if frame != nil {
		t.Error("expected nil when no PID byte available")
	}
}

func TestCreateUIFrame_InvalidSource(t *testing.T) {
	_, err := CreateUIFrame("TOOLONGCALL", "RFMP", []byte("test"))
	if err == nil {
		t.Error("expected error for invalid source")
	}
}

func TestCreateUIFrame_InvalidDestination(t *testing.T) {
	_, err := CreateUIFrame("N0CALL", "TOOLONGCALL", []byte("test"))
	if err == nil {
		t.Error("expected error for invalid destination")
	}
}

func TestAX25FrameWithDigipeaters(t *testing.T) {
	src, _ := NewAX25Address("N0CALL", 0)
	dst, _ := NewAX25Address("RFMP", 0)
	digi1, _ := NewAX25Address("WIDE1", 1)
	digi2, _ := NewAX25Address("WIDE2", 2)

	frame := &AX25Frame{
		Destination: dst,
		Source:      src,
		Digipeaters: []*AX25Address{digi1, digi2},
		Control:     0x03,
		PID:         0xF0,
		Info:        []byte("digi test"),
	}

	encoded := frame.Encode()
	decoded := DecodeAX25Frame(encoded)
	if decoded == nil {
		t.Fatal("expected decoded frame")
	}
	if decoded.Source.Callsign != "N0CALL" {
		t.Errorf("source: %s", decoded.Source.Callsign)
	}
	if decoded.Destination.Callsign != "RFMP" {
		t.Errorf("dest: %s", decoded.Destination.Callsign)
	}
	if len(decoded.Digipeaters) != 2 {
		t.Fatalf("expected 2 digipeaters, got %d", len(decoded.Digipeaters))
	}
	if decoded.Digipeaters[0].Callsign != "WIDE1" || decoded.Digipeaters[0].SSID != 1 {
		t.Errorf("digi0: %s-%d", decoded.Digipeaters[0].Callsign, decoded.Digipeaters[0].SSID)
	}
	if decoded.Digipeaters[1].Callsign != "WIDE2" || decoded.Digipeaters[1].SSID != 2 {
		t.Errorf("digi1: %s-%d", decoded.Digipeaters[1].Callsign, decoded.Digipeaters[1].SSID)
	}
	if string(decoded.Info) != "digi test" {
		t.Errorf("info: %q", string(decoded.Info))
	}
}
