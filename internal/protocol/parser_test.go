package protocol

import (
	"strings"
	"testing"
	"time"
)

func testID() [6]byte {
	return [6]byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45}
}

func testTime() time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
}

func TestEncodeMSGBasicRoundTrip(t *testing.T) {
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     testTime(),
		Channel:  "general",
		Body:     "Hello world",
	}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 {
		t.Fatal("encode returned empty")
	}

	frame, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded := frame.(*MSG)
	if decoded.ID != msg.ID {
		t.Errorf("id: got %v, want %v", decoded.ID, msg.ID)
	}
	if decoded.FromNode != "N0CALL" {
		t.Errorf("from: got %s", decoded.FromNode)
	}
	if !decoded.Time.Equal(testTime()) {
		t.Errorf("time: got %v, want %v", decoded.Time, testTime())
	}
	if decoded.Channel != "general" {
		t.Errorf("channel: got %s", decoded.Channel)
	}
	if decoded.Body != "Hello world" {
		t.Errorf("body: got %q", decoded.Body)
	}
	if decoded.ReplyTo != nil {
		t.Errorf("reply_to: should be nil")
	}
	if decoded.Seq != nil {
		t.Errorf("seq: should be nil")
	}
}

func TestEncodeMSGWithReplyAndSeq(t *testing.T) {
	replyID := [6]byte{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54}
	seq := 5
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL-1",
		Time:     testTime(),
		Channel:  "general",
		ReplyTo:  &replyID,
		Body:     "Reply here",
		Seq:      &seq,
	}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded := frame.(*MSG)
	if decoded.ReplyTo == nil {
		t.Fatal("expected reply_to")
	}
	if *decoded.ReplyTo != replyID {
		t.Errorf("reply_to: got %v, want %v", *decoded.ReplyTo, replyID)
	}
	if decoded.Seq == nil || *decoded.Seq != 5 {
		t.Errorf("seq: got %v", decoded.Seq)
	}
}

func TestEncodeMSGUTF8Body(t *testing.T) {
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     testTime(),
		Channel:  "general",
		Body:     "Café ☕ hello|world 50% off",
	}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if frame.(*MSG).Body != msg.Body {
		t.Errorf("body: got %q, want %q", frame.(*MSG).Body, msg.Body)
	}
}

func TestEncodeFRAGRoundTrip(t *testing.T) {
	frag := &FRAG{
		MessageID: testID(),
		Idx:       1,
		Total:     3,
		Data:      []byte("some raw protobuf bytes here"),
	}
	encoded, err := Encode(frag)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded := frame.(*FRAG)
	if decoded.MessageID != testID() {
		t.Errorf("msgid mismatch")
	}
	if decoded.Idx != 1 {
		t.Errorf("idx: got %d", decoded.Idx)
	}
	if decoded.Total != 3 {
		t.Errorf("total: got %d", decoded.Total)
	}
	if string(decoded.Data) != "some raw protobuf bytes here" {
		t.Errorf("data mismatch")
	}
}

func TestEncodeSVECRoundTrip(t *testing.T) {
	svec := &SVEC{
		FromNode: "N0CALL",
		Vector: map[string]int{
			"KD2ABC":   3,
			"N0CALL":   12,
			"W1AW":     7,
			"N0CALL-1": 5,
		},
	}
	encoded, err := Encode(svec)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded := frame.(*SVEC)
	if decoded.FromNode != "N0CALL" {
		t.Errorf("from: %s", decoded.FromNode)
	}
	if decoded.Vector["KD2ABC"] != 3 {
		t.Errorf("KD2ABC: got %d", decoded.Vector["KD2ABC"])
	}
	if decoded.Vector["N0CALL"] != 12 {
		t.Errorf("N0CALL: got %d", decoded.Vector["N0CALL"])
	}
	if decoded.Vector["W1AW"] != 7 {
		t.Errorf("W1AW: got %d", decoded.Vector["W1AW"])
	}
	if decoded.Vector["N0CALL-1"] != 5 {
		t.Errorf("N0CALL-1: got %d", decoded.Vector["N0CALL-1"])
	}
}

func TestDecodeInvalidData(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"garbage", []byte{0xFF, 0xFF, 0xFF}},
		{"nil", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.data)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestEncodeCompactness(t *testing.T) {
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     testTime(),
		Channel:  "general",
		Body:     "Hello world",
	}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 60 {
		t.Errorf("protobuf MSG should be compact, got %d bytes", len(encoded))
	}
}

func TestEncodeCompactnessBenchmark(t *testing.T) {
	id := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	msg := &MSG{
		ID: id, FromNode: "N0CALL",
		Time: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel: "general", Body: "Hello world",
	}
	enc1, _ := Encode(msg)
	if size := len(enc1); size > 50 {
		t.Errorf("short MSG too large: %d bytes", size)
	}

	body140 := strings.Repeat("x", 140)
	id2 := GenerateMessageID("N0CALL", 1705320000, body140)
	msg2 := &MSG{
		ID: id2, FromNode: "N0CALL",
		Time: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel: "general", Body: body140,
	}
	enc2, _ := Encode(msg2)
	if size := len(enc2); size > 185 {
		t.Errorf("140-char MSG too large: %d bytes", size)
	}

	svec := &SVEC{
		FromNode: "N0CALL",
		Vector: map[string]int{"KD2ABC": 3, "N0CALL": 12, "W1AW": 7, "AB1CDE": 22, "N0XYZ": 9},
	}
	enc3, _ := Encode(svec)
	if size := len(enc3); size > 75 {
		t.Errorf("5-node SVEC too large: %d bytes", size)
	}
}
