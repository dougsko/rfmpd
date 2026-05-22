package protocol

import (
	"testing"
	"time"
)

func TestGenerateMessageID(t *testing.T) {
	id := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	hex := IDToHex(id)
	if len(hex) != 12 {
		t.Errorf("expected 12 hex chars, got %d: %s", len(hex), hex)
	}
}

func TestGenerateMessageIDDeterministic(t *testing.T) {
	id1 := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	id2 := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
}

func TestGenerateMessageIDDifferentSender(t *testing.T) {
	id1 := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	id2 := GenerateMessageID("Doug", 1705320000, "Hello world")
	if id1 == id2 {
		t.Error("different senders should produce different IDs")
	}
}

func TestGenerateMessageIDDifferentTime(t *testing.T) {
	id1 := GenerateMessageID("N0CALL", 1705320000, "Hello world")
	id2 := GenerateMessageID("N0CALL", 1705320001, "Hello world")
	if id1 == id2 {
		t.Error("different times should produce different IDs")
	}
}

func TestIDHexRoundTrip(t *testing.T) {
	id := GenerateMessageID("N0CALL", 1705320000, "test")
	hex := IDToHex(id)
	back, err := IDFromHex(hex)
	if err != nil {
		t.Fatal(err)
	}
	if back != id {
		t.Errorf("round-trip failed: %v != %v", back, id)
	}
}

func TestIDFromHexInvalid(t *testing.T) {
	cases := []string{"", "abc", "zzzzzzzzzzzz", "abcdef01234"}
	for _, c := range cases {
		_, err := IDFromHex(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestEpochRoundTrip(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	epoch := ToEpoch(now)
	back := FromEpoch(epoch)
	if !back.Equal(now) {
		t.Errorf("epoch round-trip: got %v, want %v", back, now)
	}
}

func TestFormatTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	got := FormatTimestamp(ts)
	want := "20240115T120000Z"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParseTimestamp(t *testing.T) {
	ts, err := ParseTimestamp("20240115T120000Z")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	if !ts.Equal(want) {
		t.Errorf("got %v, want %v", ts, want)
	}
}

func TestParseTimestampInvalid(t *testing.T) {
	_, err := ParseTimestamp("2024-01-15T12:00:00Z")
	if err == nil {
		t.Error("expected error for timestamp with separators")
	}
}
