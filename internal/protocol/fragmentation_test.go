package protocol

import (
	"strings"
	"testing"
	"time"
)

func TestFragmentMessageExceedsThreshold(t *testing.T) {
	body := strings.Repeat("x", 300)
	id := GenerateMessageID("N0CALL", 1705320000, body)
	msg := &MSG{
		ID:       id,
		FromNode: "N0CALL",
		Time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel:  "general",
		Body:     body,
	}

	f := NewFragmenter(200)
	frags := f.FragmentMessage(msg)
	if len(frags) == 0 {
		t.Fatal("expected fragments")
	}
	if frags[0].Total != len(frags) {
		t.Errorf("total mismatch: header says %d, got %d frags", frags[0].Total, len(frags))
	}

	for i, frag := range frags {
		if frag.Idx != i {
			t.Errorf("frag %d: idx=%d", i, frag.Idx)
		}
		if frag.MessageID != id {
			t.Errorf("frag %d: msgid mismatch", i)
		}
	}
}

func TestFragmentMessageBelowThreshold(t *testing.T) {
	id := GenerateMessageID("N0CALL", 1705320000, "short")
	msg := &MSG{
		ID:       id,
		FromNode: "N0CALL",
		Time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel:  "general",
		Body:     "short",
	}

	f := NewFragmenter(200)
	frags := f.FragmentMessage(msg)
	if frags != nil {
		t.Errorf("expected nil for short message, got %d frags", len(frags))
	}
}

func TestFragmentReassembly(t *testing.T) {
	body := strings.Repeat("x", 300)
	id := GenerateMessageID("N0CALL", 1705320000, body)
	msg := &MSG{
		ID:       id,
		FromNode: "N0CALL",
		Time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel:  "general",
		Body:     body,
	}

	f := NewFragmenter(200)
	frags := f.FragmentMessage(msg)
	if len(frags) == 0 {
		t.Fatal("expected fragments")
	}

	var reassembled *MSG
	for _, frag := range frags {
		_, r := f.AddFragment(frag)
		if r != nil {
			reassembled = r
		}
	}

	if reassembled == nil {
		t.Fatal("expected reassembled message")
	}
	if reassembled.Body != body {
		t.Errorf("body length: got %d, want %d", len(reassembled.Body), len(body))
	}
	if reassembled.ID != id {
		t.Errorf("id mismatch")
	}
}

func TestFragmentCollectorDuplicate(t *testing.T) {
	f := NewFragmenter(200)
	msgID := [6]byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45}
	frag := &FRAG{
		MessageID: msgID,
		Idx:       0,
		Total:     3,
		Data:      []byte("test"),
	}

	isNew, _ := f.AddFragment(frag)
	if !isNew {
		t.Error("first add should be new")
	}

	isNew, _ = f.AddFragment(frag)
	if isNew {
		t.Error("duplicate add should not be new")
	}
}

func TestFragmentReassemblyOutOfOrder(t *testing.T) {
	body := strings.Repeat("y", 400)
	id := GenerateMessageID("N0CALL", 1705320000, body)
	msg := &MSG{
		ID:       id,
		FromNode: "N0CALL",
		Time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel:  "general",
		Body:     body,
	}

	f := NewFragmenter(200)
	frags := f.FragmentMessage(msg)
	if len(frags) < 2 {
		t.Fatal("expected multiple fragments")
	}

	// Add in reverse order
	var reassembled *MSG
	for i := len(frags) - 1; i >= 0; i-- {
		_, r := f.AddFragment(frags[i])
		if r != nil {
			reassembled = r
		}
	}

	if reassembled == nil {
		t.Fatal("expected reassembled message")
	}
	if reassembled.Body != body {
		t.Errorf("body mismatch after out-of-order reassembly")
	}
}

func TestFragmentCollectorIsExpired(t *testing.T) {
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	fc := NewFragmentCollector(msgID, 3)
	if fc.IsExpired() {
		t.Error("new collector should not be expired")
	}

	fc.FirstSeen = time.Now().Add(-10 * time.Minute)
	if !fc.IsExpired() {
		t.Error("collector past timeout should be expired")
	}
}

func TestFragmentCollectorGetMissingIndexes(t *testing.T) {
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	fc := NewFragmentCollector(msgID, 4)

	fc.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 4, Data: []byte("a")})
	fc.AddFragment(&FRAG{MessageID: msgID, Idx: 2, Total: 4, Data: []byte("c")})

	missing := fc.GetMissingIndexes()
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing, got %d", len(missing))
	}
	if missing[0] != 1 || missing[1] != 3 {
		t.Errorf("expected [1, 3], got %v", missing)
	}
}

func TestFragmentCollectorReassemble_Incomplete(t *testing.T) {
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	fc := NewFragmentCollector(msgID, 3)
	fc.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 3, Data: []byte("a")})

	result := fc.Reassemble()
	if result != nil {
		t.Error("expected nil for incomplete assembly")
	}
}

func TestFragmentCollectorAddFragment_WrongMessage(t *testing.T) {
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	fc := NewFragmentCollector(msgID, 3)

	otherID := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	added := fc.AddFragment(&FRAG{MessageID: otherID, Idx: 0, Total: 3, Data: []byte("x")})
	if added {
		t.Error("should reject fragment with wrong message ID")
	}
}

func TestFragmentCollectorAddFragment_WrongTotal(t *testing.T) {
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	fc := NewFragmentCollector(msgID, 3)

	added := fc.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 5, Data: []byte("x")})
	if added {
		t.Error("should reject fragment with wrong total")
	}
}

func TestFragmenterCleanupExpired(t *testing.T) {
	f := NewFragmenter(200)
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	f.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 3, Data: []byte("a")})

	// Force expiration
	key := IDToHex(msgID)
	f.Collectors[key].FirstSeen = time.Now().Add(-10 * time.Minute)

	expired := f.CleanupExpired()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}
	if expired[0] != key {
		t.Errorf("expected key %s, got %s", key, expired[0])
	}

	if len(f.Collectors) != 0 {
		t.Error("expected empty collectors after cleanup")
	}
}

func TestFragmenterCleanupExpired_NoneExpired(t *testing.T) {
	f := NewFragmenter(200)
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	f.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 3, Data: []byte("a")})

	expired := f.CleanupExpired()
	if len(expired) != 0 {
		t.Errorf("expected 0 expired, got %d", len(expired))
	}
}

func TestFragmenterAddFragment_TotalTooLarge(t *testing.T) {
	f := NewFragmenter(200)
	msgID := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	isNew, msg := f.AddFragment(&FRAG{MessageID: msgID, Idx: 0, Total: 256, Data: []byte("a")})
	if isNew {
		t.Error("should reject fragment with total > MaxFragments")
	}
	if msg != nil {
		t.Error("should not produce message")
	}
}
