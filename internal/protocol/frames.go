package protocol

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type FrameType string

const (
	FrameTypeMSG  FrameType = "MSG"
	FrameTypeFRAG FrameType = "FRAG"
	FrameTypeSVEC FrameType = "SVEC"
)

// Frame is the interface implemented by all RFMP frame types (MSG, FRAG, SVEC).
type Frame interface {
	Type() FrameType
	ToDict() map[string]string
}

// MSG represents a message frame with a unique ID, sender, timestamp, channel, and body.
type MSG struct {
	ID       [6]byte
	FromNode string
	Time     time.Time
	Channel  string
	ReplyTo  *[6]byte
	Body     string
	Seq      *int
	Author   string
}

func (m *MSG) Type() FrameType { return FrameTypeMSG }

func (m *MSG) ToDict() map[string]string {
	reply := ""
	if m.ReplyTo != nil {
		reply = hex.EncodeToString(m.ReplyTo[:])
	}
	d := map[string]string{
		"id":     IDToHex(m.ID),
		"from":   m.FromNode,
		"time":   FormatTimestamp(m.Time),
		"chan":   m.Channel,
		"reply":  reply,
		"body":   m.Body,
		"author": m.Author,
	}
	if m.Seq != nil {
		d["seq"] = strconv.Itoa(*m.Seq)
	}
	return d
}

func MSGFromDict(d map[string]string) (*MSG, error) {
	id, err := IDFromHex(d["id"])
	if err != nil {
		return nil, err
	}

	channel := d["chan"]
	if !isValidChannel(channel) {
		return nil, fmt.Errorf("channel must be ASCII without uppercase letters")
	}

	ts, err := ParseTimestamp(d["time"])
	if err != nil {
		return nil, err
	}

	msg := &MSG{
		ID:       id,
		FromNode: d["from"],
		Time:     ts,
		Channel:  channel,
		Body:     d["body"],
	}

	reply := d["reply"]
	if reply != "" && reply != "-" {
		replyID, err := IDFromHex(reply)
		if err != nil {
			return nil, fmt.Errorf("invalid reply_to ID: %s", reply)
		}
		msg.ReplyTo = &replyID
	}

	if seqStr, ok := d["seq"]; ok && seqStr != "" {
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return nil, fmt.Errorf("seq must be >= 1, got %s", seqStr)
		}
		msg.Seq = &seq
	}

	msg.Author = d["author"]

	return msg, nil
}

func isValidChannel(ch string) bool {
	for _, c := range ch {
		if c > 127 {
			return false
		}
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

// FRAG represents a fragment of a message that exceeded the transmission threshold.
type FRAG struct {
	MessageID [6]byte
	Idx       int
	Total     int
	Data      []byte
}

func (f *FRAG) Type() FrameType { return FrameTypeFRAG }

func (f *FRAG) ToDict() map[string]string {
	return map[string]string{
		"msgid": IDToHex(f.MessageID),
		"idx":   strconv.Itoa(f.Idx),
		"total": strconv.Itoa(f.Total),
		"data":  hex.EncodeToString(f.Data),
	}
}

func FRAGFromDict(d map[string]string) (*FRAG, error) {
	msgID, err := IDFromHex(d["msgid"])
	if err != nil {
		return nil, err
	}
	idx, err := strconv.Atoi(d["idx"])
	if err != nil {
		return nil, fmt.Errorf("invalid idx: %s", d["idx"])
	}
	total, err := strconv.Atoi(d["total"])
	if err != nil {
		return nil, fmt.Errorf("invalid total: %s", d["total"])
	}
	if idx < 0 || idx >= total {
		return nil, fmt.Errorf("invalid fragment index %d/%d", idx, total)
	}
	data, err := hex.DecodeString(d["data"])
	if err != nil {
		return nil, fmt.Errorf("invalid hex data: %w", err)
	}
	return &FRAG{
		MessageID: msgID,
		Idx:       idx,
		Total:     total,
		Data:      data,
	}, nil
}

// SVEC represents a state vector broadcast carrying the sender's view of network sequence numbers.
type SVEC struct {
	FromNode string
	Vector   map[string]int
}

func (s *SVEC) Type() FrameType { return FrameTypeSVEC }

func (s *SVEC) ToDict() map[string]string {
	entries := make([]string, 0, len(s.Vector))
	keys := sortedKeys(s.Vector)
	for _, k := range keys {
		entries = append(entries, fmt.Sprintf("%s:%d", k, s.Vector[k]))
	}
	return map[string]string{
		"from": s.FromNode,
		"vec":  strings.Join(entries, ","),
	}
}

func SVECFromDict(d map[string]string) (*SVEC, error) {
	fromNode := d["from"]
	if fromNode == "" {
		return nil, fmt.Errorf("from must be non-empty")
	}

	vector := make(map[string]int)
	vecStr := d["vec"]
	if vecStr != "" {
		for _, entry := range strings.Split(vecStr, ",") {
			if !strings.Contains(entry, ":") {
				continue
			}
			lastColon := strings.LastIndex(entry, ":")
			callsign := entry[:lastColon]
			seqStr := entry[lastColon+1:]
			seq, err := strconv.Atoi(seqStr)
			if err != nil {
				return nil, fmt.Errorf("invalid seq in vector: %s", entry)
			}
			vector[callsign] = seq
		}
	}

	return &SVEC{
		FromNode: fromNode,
		Vector:   vector,
	}, nil
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
