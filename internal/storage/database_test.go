package storage

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testMsg(id, from, channel, body string, seq interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        id,
		"from_node": from,
		"author":    nil,
		"timestamp": time.Now().Format(time.RFC3339),
		"channel":   channel,
		"reply_to":  nil,
		"body":      body,
		"seq":       seq,
		"raw_frame": nil,
	}
}

func TestOpen(t *testing.T) {
	db := openTestDB(t)
	count, err := db.GetMessageCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 messages, got %d", count)
	}
}

func TestSaveMessage(t *testing.T) {
	db := openTestDB(t)
	msg := testMsg("abc123", "N0CALL", "general", "hello", 1)

	isNew, err := db.SaveMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Fatal("expected isNew=true for first insert")
	}

	count, _ := db.GetMessageCount()
	if count != 1 {
		t.Fatalf("expected 1 message, got %d", count)
	}
}

func TestSaveMessage_Deduplication(t *testing.T) {
	db := openTestDB(t)
	msg := testMsg("abc123", "N0CALL", "general", "hello", 1)

	db.SaveMessage(msg)
	isNew, err := db.SaveMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatal("expected isNew=false for duplicate")
	}

	count, _ := db.GetMessageCount()
	if count != 1 {
		t.Fatalf("expected 1 message after dup, got %d", count)
	}
}

func TestGetMessage(t *testing.T) {
	db := openTestDB(t)
	msg := testMsg("msg001", "N0CALL", "general", "test body", 1)
	db.SaveMessage(msg)

	row, err := db.GetMessage("msg001")
	if err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatal("expected message, got nil")
	}
	if row.ID != "msg001" {
		t.Fatalf("expected ID msg001, got %s", row.ID)
	}
	if row.Body != "test body" {
		t.Fatalf("expected body 'test body', got %s", row.Body)
	}
}

func TestGetMessage_NotFound(t *testing.T) {
	db := openTestDB(t)
	row, err := db.GetMessage("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatal("expected nil for nonexistent message")
	}
}

func TestGetRecentMessages(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("m1", "N0CALL", "general", "one", 1))
	db.SaveMessage(testMsg("m2", "N0CALL", "general", "two", 2))
	db.SaveMessage(testMsg("m3", "N0CALL", "other", "three", 3))

	ch := "general"
	msgs, err := db.GetRecentMessages(10, &ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in 'general', got %d", len(msgs))
	}
}

func TestSaveMessageWithSeq(t *testing.T) {
	db := openTestDB(t)
	msg := testMsg("s1", "N0CALL", "general", "first", nil)

	isNew, seq, err := db.SaveMessageWithSeq(msg, "N0CALL")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Fatal("expected isNew=true")
	}
	if seq != 1 {
		t.Fatalf("expected seq=1, got %d", seq)
	}

	msg2 := testMsg("s2", "N0CALL", "general", "second", nil)
	_, seq2, _ := db.SaveMessageWithSeq(msg2, "N0CALL")
	if seq2 != 2 {
		t.Fatalf("expected seq=2, got %d", seq2)
	}
}

func TestMarkSeenIfNew(t *testing.T) {
	db := openTestDB(t)

	isNew, err := db.MarkSeenIfNew("msg1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Fatal("expected isNew=true for first mark")
	}

	isNew, err = db.MarkSeenIfNew("msg1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatal("expected isNew=false for duplicate mark")
	}
}

func TestMarkSeenIfNew_WithFragmentIndex(t *testing.T) {
	db := openTestDB(t)
	idx := 2

	isNew, _ := db.MarkSeenIfNew("msg1", &idx)
	if !isNew {
		t.Fatal("expected isNew=true")
	}

	isNew, _ = db.MarkSeenIfNew("msg1", &idx)
	if isNew {
		t.Fatal("expected isNew=false for same fragment")
	}

	idx2 := 3
	isNew, _ = db.MarkSeenIfNew("msg1", &idx2)
	if !isNew {
		t.Fatal("expected isNew=true for different fragment index")
	}
}

func TestVectorClock(t *testing.T) {
	db := openTestDB(t)

	db.SaveMessage(testMsg("a1", "NODE-A", "general", "a1", 1))
	db.SaveMessage(testMsg("a2", "NODE-A", "general", "a2", 2))
	db.SaveMessage(testMsg("a3", "NODE-A", "general", "a3", 3))
	db.SaveMessage(testMsg("b1", "NODE-B", "general", "b1", 1))
	db.SaveMessage(testMsg("b2", "NODE-B", "general", "b2", 2))

	clock, err := db.GetVectorClock()
	if err != nil {
		t.Fatal(err)
	}
	if clock["NODE-A"] != 3 {
		t.Fatalf("expected NODE-A=3, got %d", clock["NODE-A"])
	}
	if clock["NODE-B"] != 2 {
		t.Fatalf("expected NODE-B=2, got %d", clock["NODE-B"])
	}
}

func TestVectorClock_GapDetection(t *testing.T) {
	db := openTestDB(t)

	db.SaveMessage(testMsg("a1", "NODE-A", "general", "a1", 1))
	db.SaveMessage(testMsg("a2", "NODE-A", "general", "a2", 2))
	// Skip seq 3
	db.SaveMessage(testMsg("a4", "NODE-A", "general", "a4", 4))

	clock, err := db.GetVectorClock()
	if err != nil {
		t.Fatal(err)
	}
	if clock["NODE-A"] != 2 {
		t.Fatalf("expected NODE-A=2 (gap at 3), got %d", clock["NODE-A"])
	}
}

func TestGetMessagesAfterSeq(t *testing.T) {
	db := openTestDB(t)

	db.SaveMessage(testMsg("a1", "NODE-A", "general", "a1", 1))
	db.SaveMessage(testMsg("a2", "NODE-A", "general", "a2", 2))
	db.SaveMessage(testMsg("a3", "NODE-A", "general", "a3", 3))

	msgs, err := db.GetMessagesAfterSeq("NODE-A", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after seq 1, got %d", len(msgs))
	}
	if msgs[0].ID != "a2" {
		t.Fatalf("expected first result ID=a2, got %s", msgs[0].ID)
	}
}

func TestTransmissionQueue(t *testing.T) {
	db := openTestDB(t)

	err := db.QueueTransmission("MSG", `{"test":"data"}`, 0)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.GetNextTransmission()
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Fatal("expected a transmission, got nil")
	}
	if tx["frame_type"] != "MSG" {
		t.Fatalf("expected frame_type=MSG, got %v", tx["frame_type"])
	}
	if tx["frame_data"] != `{"test":"data"}` {
		t.Fatalf("unexpected frame_data: %v", tx["frame_data"])
	}
}

func TestTransmissionQueue_Empty(t *testing.T) {
	db := openTestDB(t)

	tx, err := db.GetNextTransmission()
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Fatalf("expected nil for empty queue, got %v", tx)
	}
}

func TestTransmissionQueue_FutureSchedule(t *testing.T) {
	db := openTestDB(t)

	// Schedule 1 hour in the future
	db.QueueTransmission("MSG", `{}`, 3600)

	tx, err := db.GetNextTransmission()
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Fatal("expected nil for future-scheduled item")
	}
}

func TestChannels(t *testing.T) {
	db := openTestDB(t)

	err := db.CreateChannel("test-chan")
	if err != nil {
		t.Fatal(err)
	}

	channels, err := db.GetChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0]["name"] != "test-chan" {
		t.Fatalf("expected channel name 'test-chan', got %v", channels[0]["name"])
	}
}

func TestDeleteChannel_Empty(t *testing.T) {
	db := openTestDB(t)
	db.CreateChannel("empty-chan")

	err := db.DeleteChannel("empty-chan")
	if err != nil {
		t.Fatal(err)
	}

	channels, _ := db.GetChannels()
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after delete, got %d", len(channels))
	}
}

func TestDeleteChannel_WithMessages(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("m1", "N0CALL", "has-msgs", "hello", 1))

	err := db.DeleteChannel("has-msgs")
	if err == nil {
		t.Fatal("expected error when deleting channel with messages")
	}
}

func TestSaveFragment(t *testing.T) {
	db := openTestDB(t)
	err := db.SaveFragment("frag-msg", 0, 3, []byte("chunk0"))
	if err != nil {
		t.Fatal(err)
	}
	err = db.SaveFragment("frag-msg", 1, 3, []byte("chunk1"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCleanupOldFragments(t *testing.T) {
	db := openTestDB(t)
	db.SaveFragment("old-msg", 0, 2, []byte("data"))

	// Cleanup with 0 max age removes everything
	err := db.CleanupOldFragments(0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCleanupTransmissionQueue(t *testing.T) {
	db := openTestDB(t)
	db.QueueTransmission("MSG", `{}`, 0)

	err := db.CleanupTransmissionQueue()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMarkTransmitted(t *testing.T) {
	db := openTestDB(t)
	db.QueueTransmission("MSG", `{"body":"test"}`, 0)

	tx, _ := db.GetNextTransmission()
	if tx == nil {
		t.Fatal("expected queued transmission")
	}

	id := tx["id"].(int64)
	err := db.MarkTransmitted(id)
	if err != nil {
		t.Fatal(err)
	}

	// After marking sent, next should be nil
	tx2, _ := db.GetNextTransmission()
	if tx2 != nil {
		t.Fatal("expected no pending after MarkTransmitted")
	}
}

func TestMarkTransmissionFailed(t *testing.T) {
	db := openTestDB(t)
	db.QueueTransmission("MSG", `{}`, 0)

	tx, _ := db.GetNextTransmission()
	id := tx["id"].(int64)

	// First failure should reschedule (maxRetries=3, attempts will be 1 < 2)
	err := db.MarkTransmissionFailed(id, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Fail again until it exceeds max retries
	err = db.MarkTransmissionFailed(id, 3)
	if err != nil {
		t.Fatal(err)
	}

	err = db.MarkTransmissionFailed(id, 3)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetActiveNodes(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("n1", "ALPHA", "general", "hi", 1))
	db.SaveMessage(testMsg("n2", "BRAVO", "general", "hey", 1))

	nodes, err := db.GetActiveNodes(86400)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 active nodes, got %d", len(nodes))
	}
}

func TestGetMessageForAPI(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("api1", "N0CALL", "general", "api test", 1))

	msg, err := db.GetMessageForAPI("api1")
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if msg["id"] != "api1" {
		t.Fatalf("expected id=api1, got %v", msg["id"])
	}
	if msg["body"] != "api test" {
		t.Fatalf("expected body='api test', got %v", msg["body"])
	}
}

func TestGetMessageForAPI_NotFound(t *testing.T) {
	db := openTestDB(t)

	msg, err := db.GetMessageForAPI("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatal("expected nil for nonexistent message")
	}
}

func TestGetRecentMessagesForAPI(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("r1", "N0CALL", "general", "one", 1))
	db.SaveMessage(testMsg("r2", "N0CALL", "general", "two", 2))
	db.SaveMessage(testMsg("r3", "N0CALL", "other", "three", 3))

	msgs, err := db.GetRecentMessagesForAPI(10, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	ch := "general"
	msgs, err = db.GetRecentMessagesForAPI(10, &ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in 'general', got %d", len(msgs))
	}

	node := "N0CALL"
	msgs, err = db.GetRecentMessagesForAPI(1, nil, &node)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message with limit=1, got %d", len(msgs))
	}
}

func TestCleanupSeenCache(t *testing.T) {
	db := openTestDB(t)
	db.MarkSeenIfNew("seen1", nil)
	db.MarkSeenIfNew("seen2", nil)

	// TTL large enough that nothing is cleaned
	err := db.CleanupSeenCache(3600)
	if err != nil {
		t.Fatal(err)
	}
	isNew, _ := db.MarkSeenIfNew("seen1", nil)
	if isNew {
		t.Fatal("expected isNew=false, cache entry should still exist")
	}

	// TTL in the future (negative effective cutoff) clears everything
	err = db.CleanupSeenCache(-1)
	if err != nil {
		t.Fatal(err)
	}
	isNew, _ = db.MarkSeenIfNew("seen1", nil)
	if !isNew {
		t.Fatal("expected isNew=true after cache cleanup")
	}
}

func TestMarkMessageTransmitted(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("tx1", "N0CALL", "general", "hello", 1))

	err := db.MarkMessageTransmitted("tx1")
	if err != nil {
		t.Fatal(err)
	}

	msg, _ := db.GetMessageForAPI("tx1")
	if msg["transmitted_at"] == nil {
		t.Fatal("expected transmitted_at to be set")
	}
}

func TestIncrementRebroadcastCount(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("rb1", "N0CALL", "general", "hello", 1))

	err := db.IncrementRebroadcastCount("rb1")
	if err != nil {
		t.Fatal(err)
	}

	err = db.IncrementRebroadcastCount("rb1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUserStats(t *testing.T) {
	db := openTestDB(t)

	db.UpdateUserStats("testuser")
	db.UpdateUserStats("testuser")
	// No panic or error = success (function logs errors internally)
}

func TestUpdateNodeSync(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("ns1", "SYNC-NODE", "general", "hi", 1))

	db.UpdateNodeSync("SYNC-NODE")

	nodes, _ := db.GetActiveNodes(86400)
	found := false
	for _, n := range nodes {
		if n["callsign"] == "SYNC-NODE" {
			found = true
			if n["sync_count"].(int) != 1 {
				t.Fatalf("expected sync_count=1, got %v", n["sync_count"])
			}
		}
	}
	if !found {
		t.Fatal("expected to find SYNC-NODE in active nodes")
	}
}

func TestMarkSeen(t *testing.T) {
	db := openTestDB(t)

	err := db.MarkSeen("mark1", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	idx := 2
	err = db.MarkSeen("mark2", &idx, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteChannel_NotFound(t *testing.T) {
	db := openTestDB(t)

	err := db.DeleteChannel("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent channel")
	}
	if err.Error() != "channel not found" {
		t.Fatalf("expected 'channel not found', got %s", err.Error())
	}
}

func TestGetRecentMessages_FromNodeFilter(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("fn1", "ALPHA", "general", "from alpha", 1))
	db.SaveMessage(testMsg("fn2", "BRAVO", "general", "from bravo", 2))

	node := "ALPHA"
	msgs, err := db.GetRecentMessages(10, nil, &node)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message from ALPHA, got %d", len(msgs))
	}
	if msgs[0].FromNode != "ALPHA" {
		t.Errorf("expected from ALPHA, got %s", msgs[0].FromNode)
	}
}

func TestGetRecentMessages_BothFilters(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("bf1", "ALPHA", "general", "alpha general", 1))
	db.SaveMessage(testMsg("bf2", "ALPHA", "other", "alpha other", 2))
	db.SaveMessage(testMsg("bf3", "BRAVO", "general", "bravo general", 3))

	ch := "general"
	node := "ALPHA"
	msgs, err := db.GetRecentMessages(10, &ch, &node)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestGetChannels_MultipleWithStats(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("ch1", "ALPHA", "general", "hello", 1))
	db.SaveMessage(testMsg("ch2", "BRAVO", "general", "world", 2))
	db.SaveMessage(testMsg("ch3", "ALPHA", "other-ch", "hi", 3))

	channels, err := db.GetChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) < 1 {
		t.Fatalf("expected at least 1 channel, got %d", len(channels))
	}
	// Verify channel stats are populated
	for _, ch := range channels {
		if ch["name"] == "general" {
			if ch["message_count"].(int) < 2 {
				t.Errorf("expected general to have at least 2 messages, got %v", ch["message_count"])
			}
		}
	}
}

func TestGetActiveNodes_Empty(t *testing.T) {
	db := openTestDB(t)

	nodes, err := db.GetActiveNodes(86400)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestGetMessagesAfterSeq_NoResults(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("sq1", "N0CALL", "general", "only one", 1))

	msgs, err := db.GetMessagesAfterSeq("N0CALL", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after seq 1, got %d", len(msgs))
	}
}

func TestOpenAndClose(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.db"
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Re-open existing DB (tests migration path)
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
}

func TestGetMessagesAfterSeq_Multiple(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("ms1", "NODE-X", "general", "one", 1))
	db.SaveMessage(testMsg("ms2", "NODE-X", "general", "two", 2))
	db.SaveMessage(testMsg("ms3", "NODE-X", "general", "three", 3))
	db.SaveMessage(testMsg("ms4", "NODE-Y", "general", "four", 1))

	msgs, err := db.GetMessagesAfterSeq("NODE-X", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages from NODE-X after seq 0, got %d", len(msgs))
	}

	msgs, err = db.GetMessagesAfterSeq("NODE-Y", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message from NODE-Y, got %d", len(msgs))
	}
}

func TestErrorPaths_ClosedDB(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("err1", "N0CALL", "general", "hi", 1))
	db.Close()

	// All operations on a closed DB should error or handle gracefully
	_, err := db.GetRecentMessages(10, nil, nil)
	if err == nil {
		t.Error("expected error from GetRecentMessages on closed DB")
	}

	_, err = db.GetMessagesAfterSeq("N0CALL", 0)
	if err == nil {
		t.Error("expected error from GetMessagesAfterSeq on closed DB")
	}

	_, err = db.GetActiveNodes(86400)
	if err == nil {
		t.Error("expected error from GetActiveNodes on closed DB")
	}

	_, err = db.GetChannels()
	if err == nil {
		t.Error("expected error from GetChannels on closed DB")
	}

	_, err = db.GetVectorClock()
	if err == nil {
		t.Error("expected error from GetVectorClock on closed DB")
	}

	err = db.QueueTransmission("MSG", `{}`, 0)
	if err == nil {
		t.Error("expected error from QueueTransmission on closed DB")
	}

	_, err = db.GetNextTransmission()
	if err == nil {
		t.Error("expected error from GetNextTransmission on closed DB")
	}

	_, err = db.GetMessageCount()
	if err == nil {
		t.Error("expected error from GetMessageCount on closed DB")
	}

	// These log errors internally and don't return them
	db.UpdateUserStats("user")
	db.UpdateNodeSync("NODE")
	db.CleanupTransmissionQueue()
}

func TestGetActiveNodes_WithSyncData(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("nd1", "NODE-A", "general", "hi", 1))
	db.UpdateNodeSync("NODE-A")

	nodes, err := db.GetActiveNodes(86400)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0]["sync_count"].(int) != 1 {
		t.Errorf("expected sync_count=1, got %v", nodes[0]["sync_count"])
	}
}

func TestSaveMessageWithSeq_DuplicateErrors(t *testing.T) {
	db := openTestDB(t)
	msg := testMsg("dup-seq", "N0CALL", "general", "hello", nil)

	isNew, seq, err := db.SaveMessageWithSeq(msg, "N0CALL")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew || seq != 1 {
		t.Fatalf("expected isNew=true seq=1, got %v %d", isNew, seq)
	}

	// SaveMessageWithSeq doesn't use INSERT OR IGNORE, so duplicates error
	_, _, err = db.SaveMessageWithSeq(msg, "N0CALL")
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestQueueTransmission_NegativeDelay(t *testing.T) {
	db := openTestDB(t)
	err := db.QueueTransmission("MSG", `{"test":"negative"}`, -1.0)
	if err != nil {
		t.Fatal(err)
	}
	// Should be immediately available (scheduled_at = now)
	tx, err := db.GetNextTransmission()
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Fatal("expected transmission to be immediately available")
	}
}

func TestRound64(t *testing.T) {
	if round64(1.5) != 2 {
		t.Errorf("expected 2, got %d", round64(1.5))
	}
	if round64(1.4) != 1 {
		t.Errorf("expected 1, got %d", round64(1.4))
	}
	if round64(-1.5) != -2 {
		t.Errorf("expected -2, got %d", round64(-1.5))
	}
	if round64(-1.4) != -1 {
		t.Errorf("expected -1, got %d", round64(-1.4))
	}
	if round64(0) != 0 {
		t.Errorf("expected 0, got %d", round64(0))
	}
}

func TestSaveMessage_UpdatesChannelAndNodeStats(t *testing.T) {
	db := openTestDB(t)
	db.SaveMessage(testMsg("stat1", "ALPHA", "test-ch", "first", 1))
	db.SaveMessage(testMsg("stat2", "BRAVO", "test-ch", "second", 2))

	channels, _ := db.GetChannels()
	found := false
	for _, ch := range channels {
		if ch["name"] == "test-ch" {
			found = true
			if ch["message_count"].(int) != 2 {
				t.Errorf("expected message_count=2, got %v", ch["message_count"])
			}
		}
	}
	if !found {
		t.Fatal("expected test-ch in channels")
	}

	nodes, _ := db.GetActiveNodes(86400)
	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(nodes))
	}
}
