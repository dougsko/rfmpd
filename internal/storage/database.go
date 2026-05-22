package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	// SQLite settings
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA foreign_keys=ON")
	db.Exec("PRAGMA busy_timeout=5000")

	d := &Database{db: db}
	if err := d.createTables(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) createTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			from_node TEXT NOT NULL,
			author TEXT,
			timestamp TEXT NOT NULL,
			channel TEXT NOT NULL,
			reply_to TEXT,
			body TEXT NOT NULL,
			received_at INTEGER NOT NULL,
			transmitted_at INTEGER,
			rebroadcast_count INTEGER DEFAULT 0,
			seq INTEGER,
			raw_frame TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_from_node ON messages(from_node)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_received_at ON messages(received_at)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_from_node_seq ON messages(from_node, seq)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_from_node_seq_unique ON messages(from_node, seq) WHERE seq IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS fragments (
			message_id TEXT NOT NULL,
			idx INTEGER NOT NULL,
			total INTEGER NOT NULL,
			data BLOB NOT NULL,
			received_at INTEGER NOT NULL,
			PRIMARY KEY (message_id, idx)
		)`,
		`CREATE TABLE IF NOT EXISTS seen_cache (
			message_id TEXT NOT NULL,
			fragment_idx INTEGER NOT NULL DEFAULT -1,
			seen_at INTEGER NOT NULL,
			rebroadcast INTEGER DEFAULT 0,
			PRIMARY KEY (message_id, fragment_idx)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_seen_cache_cleanup ON seen_cache(seen_at)`,
		`CREATE TABLE IF NOT EXISTS transmission_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			frame_type TEXT NOT NULL,
			frame_data TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			scheduled_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tx_queue_status ON transmission_queue(status, scheduled_at)`,
		`CREATE TABLE IF NOT EXISTS nodes (
			callsign TEXT PRIMARY KEY,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			last_sync INTEGER,
			message_count INTEGER DEFAULT 0,
			sync_count INTEGER DEFAULT 0,
			req_count INTEGER DEFAULT 0,
			metadata TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS channels (
			name TEXT PRIMARY KEY,
			first_message INTEGER NOT NULL,
			last_message INTEGER NOT NULL,
			message_count INTEGER DEFAULT 0,
			unique_nodes INTEGER DEFAULT 0,
			metadata TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			message_count INTEGER DEFAULT 0,
			last_activity TEXT,
			metadata TEXT
		)`,
	}

	for _, stmt := range statements {
		if _, err := d.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt[:50], err)
		}
	}

	d.migrate()
	return nil
}

func (d *Database) migrate() {
	// Add columns that may be missing from older databases
	d.db.Exec("ALTER TABLE messages ADD COLUMN transmitted_at INTEGER")
	d.db.Exec("ALTER TABLE messages ADD COLUMN rebroadcast_count INTEGER DEFAULT 0")
	d.db.Exec("ALTER TABLE nodes ADD COLUMN metadata TEXT")
	d.db.Exec("ALTER TABLE channels ADD COLUMN metadata TEXT")

	// Recreate seen_cache on startup to ensure clean state
	d.db.Exec("DROP TABLE IF EXISTS seen_cache")
	d.db.Exec(`CREATE TABLE IF NOT EXISTS seen_cache (
		message_id TEXT NOT NULL,
		fragment_idx INTEGER NOT NULL DEFAULT -1,
		seen_at INTEGER NOT NULL,
		rebroadcast INTEGER DEFAULT 0,
		PRIMARY KEY (message_id, fragment_idx)
	)`)
	d.db.Exec("CREATE INDEX IF NOT EXISTS idx_seen_cache_cleanup ON seen_cache(seen_at)")

	// Drop legacy tables from earlier protocol versions
	d.db.Exec("DROP TABLE IF EXISTS bloom_windows")
	d.db.Exec("DROP TABLE IF EXISTS request_tracking")
}

type MessageRow struct {
	ID             string
	FromNode       string
	Author         *string
	Timestamp      string
	Channel        string
	ReplyTo        *string
	Body           string
	ReceivedAt     int64
	TransmittedAt  *int64
	Seq            *int
}

func (d *Database) SaveMessage(msg map[string]interface{}) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`INSERT OR IGNORE INTO messages (id, from_node, author, timestamp, channel, reply_to, body, received_at, seq, raw_frame) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg["id"], msg["from_node"], msg["author"], msg["timestamp"], msg["channel"],
		msg["reply_to"], msg["body"],
		time.Now().Unix(), msg["seq"], msg["raw_frame"],
	)
	if err != nil {
		return false, err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return false, nil
	}

	d.updateChannelStats(msg["channel"].(string), msg["from_node"].(string))
	d.updateNodeStats(msg["from_node"].(string), "message")
	return true, nil
}

func (d *Database) SaveMessageWithSeq(msg map[string]interface{}, fromNode string) (bool, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var nextSeq int
	err := d.db.QueryRow("SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE from_node = ?", fromNode).Scan(&nextSeq)
	if err != nil {
		return false, 0, err
	}

	msg["seq"] = nextSeq
	now := time.Now().Unix()

	_, err = d.db.Exec(`INSERT INTO messages (id, from_node, author, timestamp, channel, reply_to, body, received_at, seq, raw_frame) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg["id"], msg["from_node"], msg["author"], msg["timestamp"], msg["channel"],
		msg["reply_to"], msg["body"], now, nextSeq, msg["raw_frame"],
	)
	if err != nil {
		return false, 0, err
	}

	d.updateChannelStats(msg["channel"].(string), fromNode)
	d.updateNodeStats(fromNode, "message")
	return true, nextSeq, nil
}

func (d *Database) GetMessage(id string) (*MessageRow, error) {
	row := d.db.QueryRow("SELECT id, from_node, author, timestamp, channel, reply_to, body, received_at, transmitted_at, seq FROM messages WHERE id = ?", id)
	msg := &MessageRow{}
	err := row.Scan(&msg.ID, &msg.FromNode, &msg.Author, &msg.Timestamp, &msg.Channel, &msg.ReplyTo, &msg.Body, &msg.ReceivedAt, &msg.TransmittedAt, &msg.Seq)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (d *Database) GetRecentMessages(limit int, channel, fromNode *string) ([]*MessageRow, error) {
	query := "SELECT id, from_node, author, timestamp, channel, reply_to, body, received_at, transmitted_at, seq FROM messages"
	var args []interface{}
	var conditions []string

	if channel != nil {
		conditions = append(conditions, "channel = ?")
		args = append(args, *channel)
	}
	if fromNode != nil {
		conditions = append(conditions, "from_node = ?")
		args = append(args, *fromNode)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY received_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*MessageRow
	for rows.Next() {
		msg := &MessageRow{}
		if err := rows.Scan(&msg.ID, &msg.FromNode, &msg.Author, &msg.Timestamp, &msg.Channel, &msg.ReplyTo, &msg.Body, &msg.ReceivedAt, &msg.TransmittedAt, &msg.Seq); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (d *Database) GetMessageCount() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	return count, err
}

func (d *Database) MarkSeen(messageID string, fragmentIdx *int, rebroadcast bool) error {
	actualIdx := -1
	if fragmentIdx != nil {
		actualIdx = *fragmentIdx
	}
	rebroadcastInt := 0
	if rebroadcast {
		rebroadcastInt = 1
	}
	_, err := d.db.Exec(`INSERT OR REPLACE INTO seen_cache (message_id, fragment_idx, seen_at, rebroadcast) VALUES (?, ?, ?, ?)`,
		messageID, actualIdx, time.Now().Unix(), rebroadcastInt)
	return err
}

func (d *Database) MarkSeenIfNew(messageID string, fragmentIdx *int) (bool, error) {
	actualIdx := -1
	if fragmentIdx != nil {
		actualIdx = *fragmentIdx
	}
	now := time.Now().Unix()
	result, err := d.db.Exec(`INSERT OR IGNORE INTO seen_cache (message_id, fragment_idx, seen_at, rebroadcast) VALUES (?, ?, ?, 0)`,
		messageID, actualIdx, now)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (d *Database) GetVectorClock() (map[string]int, error) {
	rows, err := d.db.Query("SELECT from_node, seq FROM messages WHERE seq IS NOT NULL ORDER BY from_node, seq")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build contiguous seq for each node (highest seq with no gaps below)
	seqsByNode := make(map[string][]int)
	for rows.Next() {
		var node string
		var seq int
		if err := rows.Scan(&node, &seq); err != nil {
			return nil, err
		}
		seqsByNode[node] = append(seqsByNode[node], seq)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	clock := make(map[string]int)
	for node, seqs := range seqsByNode {
		contiguous := 0
		for _, s := range seqs {
			if s == contiguous+1 {
				contiguous = s
			}
		}
		if contiguous > 0 {
			clock[node] = contiguous
		}
	}
	return clock, nil
}

func (d *Database) GetMessagesAfterSeq(fromNode string, afterSeq int) ([]*MessageRow, error) {
	rows, err := d.db.Query("SELECT id, from_node, author, timestamp, channel, reply_to, body, received_at, transmitted_at, seq FROM messages WHERE from_node = ? AND seq > ? ORDER BY seq", fromNode, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*MessageRow
	for rows.Next() {
		msg := &MessageRow{}
		if err := rows.Scan(&msg.ID, &msg.FromNode, &msg.Author, &msg.Timestamp, &msg.Channel, &msg.ReplyTo, &msg.Body, &msg.ReceivedAt, &msg.TransmittedAt, &msg.Seq); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (d *Database) QueueTransmission(frameType, frameData string, delaySeconds float64) error {
	now := time.Now().Unix()
	scheduled := now + int64(round64(delaySeconds))
	if scheduled < now {
		scheduled = now
	}
	_, err := d.db.Exec(`INSERT INTO transmission_queue (frame_type, frame_data, status, created_at, scheduled_at) VALUES (?, ?, 'pending', ?, ?)`,
		frameType, frameData, now, scheduled)
	return err
}

func (d *Database) GetNextTransmission() (map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	row := d.db.QueryRow(`SELECT id, frame_type, frame_data FROM transmission_queue WHERE status = 'pending' AND scheduled_at <= ? ORDER BY scheduled_at ASC LIMIT 1`, now)

	var id int64
	var frameType, frameData string
	err := row.Scan(&id, &frameType, &frameData)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	d.db.Exec("UPDATE transmission_queue SET status = 'transmitting' WHERE id = ?", id)

	return map[string]interface{}{
		"id":         id,
		"frame_type": frameType,
		"frame_data": frameData,
	}, nil
}

func (d *Database) MarkTransmitted(id int64) error {
	_, err := d.db.Exec("UPDATE transmission_queue SET status = 'sent' WHERE id = ?", id)
	return err
}

func (d *Database) MarkTransmissionFailed(id int64, maxRetries int) error {
	var attempts int
	d.db.QueryRow("SELECT attempts FROM transmission_queue WHERE id = ?", id).Scan(&attempts)

	if attempts >= maxRetries-1 {
		_, err := d.db.Exec("UPDATE transmission_queue SET status = 'failed', attempts = attempts + 1 WHERE id = ?", id)
		return err
	}
	now := time.Now().Unix()
	_, err := d.db.Exec("UPDATE transmission_queue SET status = 'pending', attempts = attempts + 1, scheduled_at = ? WHERE id = ?", now+60, id)
	return err
}

func (d *Database) CleanupTransmissionQueue() error {
	now := time.Now().Unix()
	if _, err := d.db.Exec("DELETE FROM transmission_queue WHERE status = 'sent' AND created_at < ?", now-3600); err != nil {
		slog.Error("cleanup: failed to delete sent items", "err", err)
	}
	if _, err := d.db.Exec("UPDATE transmission_queue SET status = 'pending' WHERE status = 'transmitting' AND scheduled_at < ?", now-60); err != nil {
		slog.Error("cleanup: failed to reset stale transmitting items", "err", err)
	}
	if _, err := d.db.Exec("DELETE FROM transmission_queue WHERE status = 'failed' AND created_at < ?", now-86400); err != nil {
		slog.Error("cleanup: failed to delete failed items", "err", err)
	}
	return nil
}

func (d *Database) GetActiveNodes(sinceSec int64) ([]map[string]interface{}, error) {
	cutoff := time.Now().Unix() - sinceSec
	rows, err := d.db.Query("SELECT callsign, first_seen, last_seen, last_sync, message_count, sync_count, req_count FROM nodes WHERE last_seen > ?", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var callsign string
		var firstSeen, lastSeen int64
		var lastSync *int64
		var msgCount, syncCount, reqCount int
		if err := rows.Scan(&callsign, &firstSeen, &lastSeen, &lastSync, &msgCount, &syncCount, &reqCount); err != nil {
			return nil, err
		}
		node := map[string]interface{}{
			"callsign":      callsign,
			"first_seen":    firstSeen,
			"last_seen":     lastSeen,
			"last_sync":     lastSync,
			"message_count": msgCount,
			"sync_count":    syncCount,
			"req_count":     reqCount,
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (d *Database) GetChannels() ([]map[string]interface{}, error) {
	rows, err := d.db.Query("SELECT name, first_message, last_message, message_count, unique_nodes FROM channels ORDER BY last_message DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []map[string]interface{}
	for rows.Next() {
		var name string
		var firstMsg, lastMsg int64
		var msgCount, uniqueNodes int
		if err := rows.Scan(&name, &firstMsg, &lastMsg, &msgCount, &uniqueNodes); err != nil {
			return nil, err
		}
		channels = append(channels, map[string]interface{}{
			"name":          name,
			"first_message": firstMsg,
			"last_message":  lastMsg,
			"message_count": msgCount,
			"unique_nodes":  uniqueNodes,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return channels, nil
}

func (d *Database) CleanupSeenCache(ttlSeconds int64) error {
	cutoff := time.Now().Unix() - ttlSeconds
	_, err := d.db.Exec("DELETE FROM seen_cache WHERE seen_at < ?", cutoff)
	return err
}

func (d *Database) updateChannelStats(channel, fromNode string) {
	now := time.Now().Unix()
	if _, err := d.db.Exec(`INSERT INTO channels (name, first_message, last_message, message_count, unique_nodes) VALUES (?, ?, ?, 1, 1) ON CONFLICT(name) DO UPDATE SET last_message = ?, message_count = message_count + 1`, channel, now, now, now); err != nil {
		slog.Error("updateChannelStats: insert/update failed", "channel", channel, "err", err)
	}
	if _, err := d.db.Exec(`UPDATE channels SET unique_nodes = (SELECT COUNT(DISTINCT from_node) FROM messages WHERE channel = ?) WHERE name = ?`, channel, channel); err != nil {
		slog.Error("updateChannelStats: unique_nodes update failed", "channel", channel, "err", err)
	}
}

func (d *Database) updateNodeStats(callsign, eventType string) {
	now := time.Now().Unix()
	var err error
	switch eventType {
	case "message":
		_, err = d.db.Exec(`INSERT INTO nodes (callsign, first_seen, last_seen, message_count, sync_count, req_count) VALUES (?, ?, ?, 1, 0, 0) ON CONFLICT(callsign) DO UPDATE SET last_seen = ?, message_count = message_count + 1`, callsign, now, now, now)
	case "sync":
		_, err = d.db.Exec(`INSERT INTO nodes (callsign, first_seen, last_seen, last_sync, message_count, sync_count, req_count) VALUES (?, ?, ?, ?, 0, 1, 0) ON CONFLICT(callsign) DO UPDATE SET last_seen = ?, last_sync = ?, sync_count = sync_count + 1`, callsign, now, now, now, now, now)
	}
	if err != nil {
		slog.Error("updateNodeStats failed", "callsign", callsign, "event", eventType, "err", err)
	}
}

func (d *Database) GetMessageForAPI(id string) (map[string]interface{}, error) {
	msg, err := d.GetMessage(id)
	if err != nil || msg == nil {
		return nil, err
	}
	return msgRowToMap(msg), nil
}

func (d *Database) GetRecentMessagesForAPI(limit int, channel, fromNode *string) ([]map[string]interface{}, error) {
	rows, err := d.GetRecentMessages(limit, channel, fromNode)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		result = append(result, msgRowToMap(row))
	}
	return result, nil
}

func msgRowToMap(msg *MessageRow) map[string]interface{} {
	m := map[string]interface{}{
		"id":        msg.ID,
		"from_node": msg.FromNode,
		"author":    msg.Author,
		"timestamp": msg.Timestamp,
		"channel":   msg.Channel,
		"reply_to":  msg.ReplyTo,
		"body":      msg.Body,
		"received_at": time.Unix(msg.ReceivedAt, 0).Format(time.RFC3339),
	}
	if msg.TransmittedAt != nil {
		m["transmitted_at"] = time.Unix(*msg.TransmittedAt, 0).Format(time.RFC3339)
	} else {
		m["transmitted_at"] = nil
	}
	return m
}

// Fragment persistence

func (d *Database) SaveFragment(messageID string, idx, total int, data []byte) error {
	_, err := d.db.Exec(`INSERT OR IGNORE INTO fragments (message_id, idx, total, data, received_at) VALUES (?, ?, ?, ?, ?)`,
		messageID, idx, total, data, time.Now().Unix())
	return err
}

func (d *Database) CleanupOldFragments(maxAgeSec int64) error {
	cutoff := time.Now().Unix() - maxAgeSec
	_, err := d.db.Exec("DELETE FROM fragments WHERE received_at < ?", cutoff)
	return err
}

// User stats tracking

func (d *Database) UpdateUserStats(username string) {
	now := time.Now().Unix()
	if _, err := d.db.Exec(`INSERT INTO users (username, first_seen, last_seen, message_count) VALUES (?, ?, ?, 1)
		ON CONFLICT(username) DO UPDATE SET last_seen = ?, message_count = message_count + 1`,
		username, now, now, now); err != nil {
		slog.Error("UpdateUserStats failed", "username", username, "err", err)
	}
}

// UpdateNodeSync updates sync-specific stats for a node
func (d *Database) UpdateNodeSync(callsign string) {
	d.updateNodeStats(callsign, "sync")
}

// Channel creation

func (d *Database) CreateChannel(name string) error {
	now := time.Now().Unix()
	_, err := d.db.Exec(`INSERT OR IGNORE INTO channels (name, first_message, last_message, message_count, unique_nodes) VALUES (?, ?, ?, 0, 0)`,
		name, now, now)
	return err
}

func (d *Database) DeleteChannel(name string) error {
	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM messages WHERE channel = ?", name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("channel has messages")
	}
	result, err := d.db.Exec("DELETE FROM channels WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

// Mark message as transmitted

func (d *Database) MarkMessageTransmitted(id string) error {
	now := time.Now().Unix()
	_, err := d.db.Exec("UPDATE messages SET transmitted_at = ? WHERE id = ?", now, id)
	return err
}

func (d *Database) IncrementRebroadcastCount(id string) error {
	_, err := d.db.Exec("UPDATE messages SET rebroadcast_count = rebroadcast_count + 1 WHERE id = ?", id)
	return err
}

func round64(f float64) int64 {
	if f < 0 {
		return int64(f - 0.5)
	}
	return int64(f + 0.5)
}
