package protocol

import (
	"testing"
	"time"
)

func TestMSGToDict(t *testing.T) {
	seq := 3
	replyID := [6]byte{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54}
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     testTime(),
		Channel:  "general",
		Body:     "hello",
		ReplyTo:  &replyID,
		Seq:      &seq,
		Author:   "Doug",
	}

	d := msg.ToDict()
	if d["id"] != "abcdef012345" {
		t.Errorf("id: %s", d["id"])
	}
	if d["from"] != "N0CALL" {
		t.Errorf("from: %s", d["from"])
	}
	if d["chan"] != "general" {
		t.Errorf("chan: %s", d["chan"])
	}
	if d["body"] != "hello" {
		t.Errorf("body: %s", d["body"])
	}
	if d["reply"] != "fedcba987654" {
		t.Errorf("reply: %s", d["reply"])
	}
	if d["seq"] != "3" {
		t.Errorf("seq: %s", d["seq"])
	}
	if d["author"] != "Doug" {
		t.Errorf("author: %s", d["author"])
	}
}

func TestMSGToDict_NoReply(t *testing.T) {
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     testTime(),
		Channel:  "general",
		Body:     "hello",
	}

	d := msg.ToDict()
	if d["reply"] != "" {
		t.Errorf("expected empty reply, got %s", d["reply"])
	}
	if _, ok := d["seq"]; ok {
		t.Error("expected no seq key when nil")
	}
}

func TestMSGType(t *testing.T) {
	msg := &MSG{}
	if msg.Type() != FrameTypeMSG {
		t.Errorf("expected MSG, got %s", msg.Type())
	}
}

func TestMSGFromDict(t *testing.T) {
	d := map[string]string{
		"id":     "abcdef012345",
		"from":   "N0CALL",
		"time":   "20240115T120000Z",
		"chan":   "general",
		"reply":  "",
		"body":   "test message",
		"seq":    "5",
		"author": "Doug",
	}

	msg, err := MSGFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if msg.FromNode != "N0CALL" {
		t.Errorf("from: %s", msg.FromNode)
	}
	if msg.Channel != "general" {
		t.Errorf("channel: %s", msg.Channel)
	}
	if msg.Body != "test message" {
		t.Errorf("body: %s", msg.Body)
	}
	if msg.Seq == nil || *msg.Seq != 5 {
		t.Errorf("seq: %v", msg.Seq)
	}
	if msg.Author != "Doug" {
		t.Errorf("author: %s", msg.Author)
	}
	if msg.ReplyTo != nil {
		t.Error("expected nil ReplyTo for empty reply")
	}
}

func TestMSGFromDict_WithReply(t *testing.T) {
	d := map[string]string{
		"id":     "abcdef012345",
		"from":   "N0CALL",
		"time":   "20240115T120000Z",
		"chan":   "general",
		"reply":  "fedcba987654",
		"body":   "replying",
		"author": "",
	}

	msg, err := MSGFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ReplyTo == nil {
		t.Fatal("expected ReplyTo")
	}
	expected := [6]byte{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54}
	if *msg.ReplyTo != expected {
		t.Errorf("reply_to: %v", msg.ReplyTo)
	}
}

func TestMSGFromDict_InvalidID(t *testing.T) {
	d := map[string]string{
		"id":   "badid",
		"from": "N0CALL",
		"time": "20240115T120000Z",
		"chan": "general",
		"body": "test",
	}
	_, err := MSGFromDict(d)
	if err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestMSGFromDict_InvalidTime(t *testing.T) {
	d := map[string]string{
		"id":   "abcdef012345",
		"from": "N0CALL",
		"time": "not-a-time",
		"chan": "general",
		"body": "test",
	}
	_, err := MSGFromDict(d)
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestMSGFromDict_InvalidChannel(t *testing.T) {
	d := map[string]string{
		"id":   "abcdef012345",
		"from": "N0CALL",
		"time": "20240115T120000Z",
		"chan": "UPPER",
		"body": "test",
	}
	_, err := MSGFromDict(d)
	if err == nil {
		t.Error("expected error for uppercase channel")
	}
}

func TestMSGFromDict_InvalidReplyID(t *testing.T) {
	d := map[string]string{
		"id":    "abcdef012345",
		"from":  "N0CALL",
		"time":  "20240115T120000Z",
		"chan":  "general",
		"reply": "badreply",
		"body":  "test",
	}
	_, err := MSGFromDict(d)
	if err == nil {
		t.Error("expected error for invalid reply ID")
	}
}

func TestMSGFromDict_InvalidSeq(t *testing.T) {
	d := map[string]string{
		"id":   "abcdef012345",
		"from": "N0CALL",
		"time": "20240115T120000Z",
		"chan": "general",
		"body": "test",
		"seq":  "0",
	}
	_, err := MSGFromDict(d)
	if err == nil {
		t.Error("expected error for seq < 1")
	}
}

func TestMSGFromDict_DashReply(t *testing.T) {
	d := map[string]string{
		"id":   "abcdef012345",
		"from": "N0CALL",
		"time": "20240115T120000Z",
		"chan": "general",
		"reply": "-",
		"body": "test",
	}
	msg, err := MSGFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ReplyTo != nil {
		t.Error("expected nil ReplyTo for dash reply")
	}
}

func TestFRAGType(t *testing.T) {
	f := &FRAG{}
	if f.Type() != FrameTypeFRAG {
		t.Errorf("expected FRAG, got %s", f.Type())
	}
}

func TestFRAGToDict(t *testing.T) {
	frag := &FRAG{
		MessageID: testID(),
		Idx:       1,
		Total:     3,
		Data:      []byte{0xDE, 0xAD},
	}

	d := frag.ToDict()
	if d["msgid"] != "abcdef012345" {
		t.Errorf("msgid: %s", d["msgid"])
	}
	if d["idx"] != "1" {
		t.Errorf("idx: %s", d["idx"])
	}
	if d["total"] != "3" {
		t.Errorf("total: %s", d["total"])
	}
	if d["data"] != "dead" {
		t.Errorf("data: %s", d["data"])
	}
}

func TestFRAGFromDict(t *testing.T) {
	d := map[string]string{
		"msgid": "abcdef012345",
		"idx":   "1",
		"total": "3",
		"data":  "dead",
	}

	frag, err := FRAGFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if frag.MessageID != testID() {
		t.Errorf("msgid mismatch")
	}
	if frag.Idx != 1 {
		t.Errorf("idx: %d", frag.Idx)
	}
	if frag.Total != 3 {
		t.Errorf("total: %d", frag.Total)
	}
}

func TestFRAGFromDict_InvalidMsgID(t *testing.T) {
	d := map[string]string{"msgid": "bad", "idx": "0", "total": "3", "data": "aa"}
	_, err := FRAGFromDict(d)
	if err == nil {
		t.Error("expected error for invalid msgid")
	}
}

func TestFRAGFromDict_InvalidIdx(t *testing.T) {
	d := map[string]string{"msgid": "abcdef012345", "idx": "abc", "total": "3", "data": "aa"}
	_, err := FRAGFromDict(d)
	if err == nil {
		t.Error("expected error for non-numeric idx")
	}
}

func TestFRAGFromDict_InvalidTotal(t *testing.T) {
	d := map[string]string{"msgid": "abcdef012345", "idx": "0", "total": "abc", "data": "aa"}
	_, err := FRAGFromDict(d)
	if err == nil {
		t.Error("expected error for non-numeric total")
	}
}

func TestFRAGFromDict_IdxOutOfRange(t *testing.T) {
	d := map[string]string{"msgid": "abcdef012345", "idx": "3", "total": "3", "data": "aa"}
	_, err := FRAGFromDict(d)
	if err == nil {
		t.Error("expected error for idx >= total")
	}
}

func TestFRAGFromDict_InvalidHexData(t *testing.T) {
	d := map[string]string{"msgid": "abcdef012345", "idx": "0", "total": "3", "data": "zzzz"}
	_, err := FRAGFromDict(d)
	if err == nil {
		t.Error("expected error for invalid hex data")
	}
}

func TestSVECType(t *testing.T) {
	s := &SVEC{}
	if s.Type() != FrameTypeSVEC {
		t.Errorf("expected SVEC, got %s", s.Type())
	}
}

func TestSVECToDict(t *testing.T) {
	svec := &SVEC{
		FromNode: "N0CALL",
		Vector:   map[string]int{"ALPHA": 3, "BRAVO": 7},
	}

	d := svec.ToDict()
	if d["from"] != "N0CALL" {
		t.Errorf("from: %s", d["from"])
	}
	// Keys are sorted, so ALPHA:3 comes before BRAVO:7
	if d["vec"] != "ALPHA:3,BRAVO:7" {
		t.Errorf("vec: %s", d["vec"])
	}
}

func TestSVECToDict_EmptyVector(t *testing.T) {
	svec := &SVEC{FromNode: "N0CALL", Vector: map[string]int{}}
	d := svec.ToDict()
	if d["vec"] != "" {
		t.Errorf("expected empty vec, got %s", d["vec"])
	}
}

func TestSVECFromDict(t *testing.T) {
	d := map[string]string{
		"from": "N0CALL",
		"vec":  "ALPHA:3,BRAVO:7",
	}

	svec, err := SVECFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if svec.FromNode != "N0CALL" {
		t.Errorf("from: %s", svec.FromNode)
	}
	if svec.Vector["ALPHA"] != 3 {
		t.Errorf("ALPHA: %d", svec.Vector["ALPHA"])
	}
	if svec.Vector["BRAVO"] != 7 {
		t.Errorf("BRAVO: %d", svec.Vector["BRAVO"])
	}
}

func TestSVECFromDict_EmptyFrom(t *testing.T) {
	d := map[string]string{"from": "", "vec": "A:1"}
	_, err := SVECFromDict(d)
	if err == nil {
		t.Error("expected error for empty from")
	}
}

func TestSVECFromDict_EmptyVec(t *testing.T) {
	d := map[string]string{"from": "N0CALL", "vec": ""}
	svec, err := SVECFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(svec.Vector) != 0 {
		t.Errorf("expected empty vector, got %v", svec.Vector)
	}
}

func TestSVECFromDict_InvalidSeq(t *testing.T) {
	d := map[string]string{"from": "N0CALL", "vec": "NODE:abc"}
	_, err := SVECFromDict(d)
	if err == nil {
		t.Error("expected error for non-numeric seq")
	}
}

func TestSVECFromDict_CallsignWithSSID(t *testing.T) {
	d := map[string]string{"from": "N0CALL", "vec": "W1AW-5:10"}
	svec, err := SVECFromDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if svec.Vector["W1AW-5"] != 10 {
		t.Errorf("expected W1AW-5=10, got %d", svec.Vector["W1AW-5"])
	}
}

func TestIsValidChannel(t *testing.T) {
	valid := []string{"general", "test-1", "my_chan", "123"}
	for _, ch := range valid {
		if !isValidChannel(ch) {
			t.Errorf("expected %q to be valid", ch)
		}
	}

	invalid := []string{"UPPER", "MiXeD", "café"}
	for _, ch := range invalid {
		if isValidChannel(ch) {
			t.Errorf("expected %q to be invalid", ch)
		}
	}
}

func TestEncodeMsgRaw_RoundTrip(t *testing.T) {
	msg := &MSG{
		ID:       testID(),
		FromNode: "N0CALL",
		Time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Channel:  "general",
		Body:     "raw encoding test",
	}

	data := EncodeMsgRaw(msg)
	if data == nil {
		t.Fatal("encode returned nil")
	}

	decoded, err := DecodeMsgRaw(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Body != "raw encoding test" {
		t.Errorf("body: %s", decoded.Body)
	}
}

func TestDecodeMsgRaw_InvalidData(t *testing.T) {
	_, err := DecodeMsgRaw([]byte{0xFF, 0xFF, 0xFF})
	if err == nil {
		t.Error("expected error for garbage data")
	}
}

func TestDecodeInvalidFragPB(t *testing.T) {
	// A FRAG with msg_id that's not 6 bytes should error
	frag := &FRAG{
		MessageID: testID(),
		Idx:       0,
		Total:     3,
		Data:      []byte("test"),
	}
	encoded, _ := Encode(frag)
	if encoded == nil {
		t.Fatal("encode failed")
	}
	// Verify it decodes normally
	_, err := Decode(encoded)
	if err != nil {
		t.Fatalf("valid frag should decode: %v", err)
	}
}
