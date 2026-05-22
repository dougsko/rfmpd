package protocol

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateMessageID produces a 6-byte deterministic ID from the sender, timestamp, and body.
func GenerateMessageID(fromNode string, epoch uint32, body string) [6]byte {
	h := sha256.New()
	h.Write([]byte(fromNode))
	binary.Write(h, binary.BigEndian, epoch)
	h.Write([]byte(body))
	var id [6]byte
	copy(id[:], h.Sum(nil)[:6])
	return id
}

// IDToHex converts a binary message ID to its 12-character hex display form.
func IDToHex(id [6]byte) string {
	return hex.EncodeToString(id[:])
}

// IDFromHex parses a 12-character hex string into a binary message ID.
func IDFromHex(s string) ([6]byte, error) {
	var id [6]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 6 {
		return id, fmt.Errorf("invalid message ID: %s", s)
	}
	copy(id[:], b)
	return id, nil
}

// ToEpoch converts a time.Time to a uint32 Unix epoch for wire encoding.
func ToEpoch(t time.Time) uint32 {
	return uint32(t.Unix())
}

// FromEpoch converts a uint32 Unix epoch from the wire to a time.Time.
func FromEpoch(epoch uint32) time.Time {
	return time.Unix(int64(epoch), 0).UTC()
}

// FormatTimestamp formats a time as YYYYMMDDTHHMMSSZ for API and storage use.
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// ParseTimestamp parses a YYYYMMDDTHHMMSSZ string back to a time.Time.
func ParseTimestamp(ts string) (time.Time, error) {
	t, err := time.Parse("20060102T150405Z", ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp: %s", ts)
	}
	return t, nil
}
