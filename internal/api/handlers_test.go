package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"
)

type mockDaemon struct {
	db          *mockDB
	connected   bool
	sendErr     error
	statsErr    error
	lastChannel string
	lastBody    string
}

func (m *mockDaemon) SendMessage(channel, body string, replyTo *string, author *string) (map[string]interface{}, error) {
	m.lastChannel = channel
	m.lastBody = body
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return map[string]interface{}{
		"id":        "test-id-123",
		"from_node": "N0CALL",
		"channel":   channel,
		"body":      body,
	}, nil
}

func (m *mockDaemon) GetStats() (map[string]interface{}, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return map[string]interface{}{
		"uptime_seconds": 42,
		"messages_sent":  5,
	}, nil
}

func (m *mockDaemon) GetConfig() (string, int, string) {
	return "N0CALL", 0, "N0CALL-0"
}

func (m *mockDaemon) SetCallsign(callsign string, ssid int) error {
	return nil
}

func (m *mockDaemon) GetDB() interface{} {
	return m.db
}

func (m *mockDaemon) IsConnected() bool {
	return m.connected
}

type mockDB struct {
	messages    []map[string]interface{}
	channels    []map[string]interface{}
	nodes       []map[string]interface{}
	msgByID     map[string]map[string]interface{}
	createErr   error
	deleteErr   error
	nodesErr    error
	messagesErr error
	channelsErr error
	msgByIDErr  error
	countErr    error
}

func (m *mockDB) GetMessageForAPI(id string) (map[string]interface{}, error) {
	if m.msgByIDErr != nil {
		return nil, m.msgByIDErr
	}
	if msg, ok := m.msgByID[id]; ok {
		return msg, nil
	}
	return nil, nil
}

func (m *mockDB) GetRecentMessagesForAPI(limit int, channel, fromNode *string) ([]map[string]interface{}, error) {
	if m.messagesErr != nil {
		return nil, m.messagesErr
	}
	return m.messages, nil
}

func (m *mockDB) GetMessageCount() (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return len(m.messages), nil
}

func (m *mockDB) GetActiveNodes(sinceSec int64) ([]map[string]interface{}, error) {
	if m.nodesErr != nil {
		return nil, m.nodesErr
	}
	return m.nodes, nil
}

func (m *mockDB) GetChannels() ([]map[string]interface{}, error) {
	if m.channelsErr != nil {
		return nil, m.channelsErr
	}
	return m.channels, nil
}

func (m *mockDB) CreateChannel(name string) error {
	return m.createErr
}

func (m *mockDB) DeleteChannel(name string) error {
	return m.deleteErr
}

func newTestServer() (*Server, *mockDaemon) {
	db := &mockDB{
		msgByID: make(map[string]map[string]interface{}),
	}
	daemon := &mockDaemon{db: db, connected: true}
	s := NewServer(daemon, []string{"*"}, nil, nil, "0.5.0-test")
	return s, daemon
}

func TestHealthEndpoint(t *testing.T) {
	s, _ := newTestServer()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %s", resp["status"])
	}
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()
	req := httptest.NewRequest("POST", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestGetMessages(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.messages = []map[string]interface{}{
		{"id": "m1", "body": "hello"},
	}

	req := httptest.NewRequest("GET", "/messages", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var msgs []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &msgs)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestGetMessages_EmptyReturnsArray(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/messages", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := strings.TrimSpace(w.Body.String())
	if body != "[]" {
		t.Fatalf("expected empty array '[]', got %s", body)
	}
}

func TestPostMessage(t *testing.T) {
	s, daemon := newTestServer()

	body := `{"channel": "general", "body": "hello world"}`
	req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if daemon.lastChannel != "general" {
		t.Fatalf("expected channel=general, got %s", daemon.lastChannel)
	}
}

func TestPostMessage_MissingFields(t *testing.T) {
	s, _ := newTestServer()

	body := `{"channel": "general"}`
	req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestPostMessage_InvalidChannel(t *testing.T) {
	s, _ := newTestServer()

	tests := []string{
		`{"channel": "UPPER", "body": "x"}`,
		`{"channel": "has spaces", "body": "x"}`,
		`{"channel": "toolongchannelname12345", "body": "x"}`,
		`{"channel": "", "body": "x"}`,
	}

	for _, body := range tests {
		req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)

		if w.Code != 422 {
			t.Fatalf("expected 422 for %s, got %d", body, w.Code)
		}
	}
}

func TestPostMessage_BodyTooLong(t *testing.T) {
	s, _ := newTestServer()

	longBody := strings.Repeat("x", 10001)
	body := fmt.Sprintf(`{"channel": "general", "body": "%s"}`, longBody)
	req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestGetMessageByID(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.msgByID["abc"] = map[string]interface{}{"id": "abc", "body": "found"}

	req := httptest.NewRequest("GET", "/messages/abc", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMessageByID_NotFound(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/messages/nonexistent", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["version"] != "0.5.0-test" {
		t.Fatalf("expected version=0.5.0-test, got %v", resp["version"])
	}
}

func TestGetChannels(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.channels = []map[string]interface{}{
		{"name": "general", "message_count": 5},
	}

	req := httptest.NewRequest("GET", "/channels", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetCallsign(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/config/callsign", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["callsign"] != "N0CALL" {
		t.Fatalf("expected callsign=N0CALL, got %v", resp["callsign"])
	}
}

func TestSetCallsign(t *testing.T) {
	s, _ := newTestServer()

	body := `{"callsign": "W1AW", "ssid": 5}`
	req := httptest.NewRequest("POST", "/config/callsign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetCallsign_Invalid(t *testing.T) {
	s, _ := newTestServer()

	tests := []string{
		`{"callsign": "toolongcall", "ssid": 0}`,
		`{"callsign": "W1AW", "ssid": 16}`,
		`{"callsign": "W1AW", "ssid": -1}`,
	}

	for _, body := range tests {
		req := httptest.NewRequest("POST", "/config/callsign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)

		if w.Code != 422 {
			t.Fatalf("expected 422 for %s, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

func TestGetNodes(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.nodes = []map[string]interface{}{
		{"callsign": "W1AW", "first_seen": int64(1000), "last_seen": int64(2000), "last_sync": (*int64)(nil), "message_count": 5, "sync_count": 1, "req_count": 0},
	}

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var nodes []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &nodes)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
}

func TestGetNodes_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/nodes", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestGetMessages_WithQueryParams(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.messages = []map[string]interface{}{
		{"id": "m1", "body": "hello", "channel": "general"},
	}

	req := httptest.NewRequest("GET", "/messages?channel=general&from_node=W1AW&limit=50", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPostMessage_SendError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.sendErr = fmt.Errorf("radio disconnected")

	body := `{"channel": "general", "body": "hello"}`
	req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestPostMessage_InvalidJSON(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/messages", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestCreateChannel(t *testing.T) {
	s, _ := newTestServer()

	body := `{"name": "new-chan"}`
	req := httptest.NewRequest("POST", "/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateChannel_InvalidName(t *testing.T) {
	s, _ := newTestServer()

	tests := []string{
		`{"name": "has spaces"}`,
		`{"name": ""}`,
		`{"name": "waytoolongchannelname123"}`,
		`{"name": "bad!chars"}`,
	}

	for _, body := range tests {
		req := httptest.NewRequest("POST", "/channels", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)

		if w.Code != 422 {
			t.Fatalf("expected 422 for %s, got %d", body, w.Code)
		}
	}
}

func TestCreateChannel_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.createErr = fmt.Errorf("db error")

	body := `{"name": "fail-chan"}`
	req := httptest.NewRequest("POST", "/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDeleteChannel(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/channels/empty-chan", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteChannel_HasMessages(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.deleteErr = fmt.Errorf("channel has messages")

	req := httptest.NewRequest("DELETE", "/channels/busy-chan", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 409 {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteChannel_NotFound(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.deleteErr = fmt.Errorf("channel not found")

	req := httptest.NewRequest("DELETE", "/channels/ghost", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMessagesMethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/messages", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestChannelsMethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/channels", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestChannelByName_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/channels/some-chan", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Fatalf("expected Allow-Origin=http://example.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("expected no Allow-Credentials with wildcard config, got %s", w.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("OPTIONS", "/messages", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected Allow-Methods header on preflight")
	}
}

func TestStatus_Disconnected(t *testing.T) {
	s, daemon := newTestServer()
	daemon.connected = false

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["connected_to_direwolf"] != false {
		t.Fatalf("expected connected_to_direwolf=false, got %v", resp["connected_to_direwolf"])
	}
}

func TestMessageByID_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/messages/abc", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestGetNodes_WithActiveHours(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.nodes = []map[string]interface{}{
		{"callsign": "W1AW", "first_seen": int64(1000), "last_seen": int64(2000), "last_sync": (*int64)(nil), "message_count": 5, "sync_count": 1, "req_count": 0},
	}

	req := httptest.NewRequest("GET", "/nodes?active_hours=48", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetNodes_WithLastSync(t *testing.T) {
	s, daemon := newTestServer()
	syncTime := int64(3000)
	daemon.db.nodes = []map[string]interface{}{
		{"callsign": "W1AW", "first_seen": int64(1000), "last_seen": int64(2000), "last_sync": &syncTime, "message_count": 5, "sync_count": 1, "req_count": 0},
	}

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var nodes []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &nodes)
	if nodes[0]["last_sync"] == nil {
		t.Fatal("expected non-nil last_sync")
	}
}

func TestGetChannels_WithData(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.channels = []map[string]interface{}{
		{"name": "general", "first_message": int64(1000), "last_message": int64(2000), "message_count": 10, "unique_nodes": 3},
	}

	req := httptest.NewRequest("GET", "/channels", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var channels []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &channels)
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0]["name"] != "general" {
		t.Errorf("expected name=general, got %v", channels[0]["name"])
	}
}

func TestGetCallsign_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/config/callsign", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSetCallsign_InvalidJSON(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/config/callsign", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestStatus_MethodNotAllowed(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCORS_NoOrigin(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS headers without Origin")
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	db := &mockDB{msgByID: make(map[string]map[string]interface{})}
	daemon := &mockDaemon{db: db, connected: true}
	s := NewServer(daemon, []string{"http://myapp.com"}, nil, nil, "0.5.0-test")

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://myapp.com")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://myapp.com" {
		t.Fatalf("expected Allow-Origin=http://myapp.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("expected Allow-Credentials with specific origin")
	}
}

func TestCORS_UnallowedOrigin(t *testing.T) {
	db := &mockDB{msgByID: make(map[string]map[string]interface{})}
	daemon := &mockDaemon{db: db, connected: true}
	s := NewServer(daemon, []string{"http://myapp.com"}, nil, nil, "0.5.0-test")

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS headers for unallowed origin")
	}
}

func TestPostMessage_WithReplyTo(t *testing.T) {
	s, daemon := newTestServer()

	body := `{"channel": "general", "body": "hello", "reply_to": "abc123", "author": "Doug"}`
	req := httptest.NewRequest("POST", "/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if daemon.lastBody != "hello" {
		t.Errorf("expected body=hello, got %s", daemon.lastBody)
	}
}

func TestGetMessages_LimitParam(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.messages = []map[string]interface{}{
		{"id": "m1", "body": "hello"},
	}

	req := httptest.NewRequest("GET", "/messages?limit=5", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMessages_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.messagesErr = fmt.Errorf("db connection lost")

	req := httptest.NewRequest("GET", "/messages", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetNodes_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.nodesErr = fmt.Errorf("db error")

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetChannels_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.channelsErr = fmt.Errorf("db error")

	req := httptest.NewRequest("GET", "/channels", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetMessageByID_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.msgByIDErr = fmt.Errorf("db error")

	req := httptest.NewRequest("GET", "/messages/abc", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestFormatUnix(t *testing.T) {
	ts := int64(1705320000)
	result := formatUnix(ts)
	if result == "" {
		t.Fatal("expected non-empty result for int64")
	}

	ptrTs := &ts
	result = formatUnix(ptrTs)
	if result == "" {
		t.Fatal("expected non-empty result for *int64")
	}

	var nilPtr *int64
	result = formatUnix(nilPtr)
	if result != "" {
		t.Fatalf("expected empty for nil *int64, got %s", result)
	}

	intVal := int(1705320000)
	result = formatUnix(intVal)
	if result == "" {
		t.Fatal("expected non-empty result for int")
	}

	result = formatUnix("not a number")
	if result != "" {
		t.Fatalf("expected empty for string, got %s", result)
	}
}

func TestCreateChannel_InvalidJSON(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("POST", "/channels", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestDeleteChannel_DBError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.db.deleteErr = fmt.Errorf("internal db error")

	req := httptest.NewRequest("DELETE", "/channels/test", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStatus_StatsError(t *testing.T) {
	s, daemon := newTestServer()
	daemon.statsErr = fmt.Errorf("stats unavailable")

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestChannelByName_EmptyName(t *testing.T) {
	s, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/channels/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSetCallsign_EmptyCallsign(t *testing.T) {
	s, _ := newTestServer()

	body := `{"callsign": "", "ssid": 0}`
	req := httptest.NewRequest("POST", "/config/callsign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func newTestServerWithFS() (*Server, *mockDaemon) {
	db := &mockDB{msgByID: make(map[string]map[string]interface{})}
	daemon := &mockDaemon{db: db, connected: true}
	webFS := fstest.MapFS{
		"index.html":                {Data: []byte("<html>test</html>")},
		"static/app.js":            {Data: []byte("console.log('hi')")},
		"static/images/favicon.svg": {Data: []byte("<svg/>")},
	}
	logger := slog.Default()
	s := NewServer(daemon, []string{"*"}, fs.FS(webFS), logger, "0.5.0-test")
	return s, daemon
}

func TestStaticIndex(t *testing.T) {
	s, _ := newTestServerWithFS()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<html>test</html>") {
		t.Fatalf("expected index HTML, got %s", w.Body.String())
	}
}

func TestStaticFile(t *testing.T) {
	s, _ := newTestServerWithFS()

	req := httptest.NewRequest("GET", "/static/app.js", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFavicon(t *testing.T) {
	s, _ := newTestServerWithFS()

	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("expected svg content type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestNotFoundPath(t *testing.T) {
	s, _ := newTestServerWithFS()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWebSocket_Stream(t *testing.T) {
	s, _ := newTestServerWithFS()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/stream"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp: %v)", err, resp)
	}
	defer conn.Close()

	// Broadcast a message
	s.BroadcastMessage(map[string]interface{}{"id": "test", "body": "hello"})

	// Read the broadcast
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	if msg["type"] != "message" {
		t.Fatalf("expected type=message, got %v", msg["type"])
	}
}

func TestWebSocket_Disconnect(t *testing.T) {
	s, _ := newTestServerWithFS()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Close the connection from client side
	conn.Close()

	// Broadcast should handle disconnected client gracefully
	s.BroadcastMessage(map[string]interface{}{"id": "test", "body": "after disconnect"})
}

func TestBroadcastMessage_NoClients(t *testing.T) {
	s, _ := newTestServer()
	// Should not panic
	s.BroadcastMessage(map[string]interface{}{"id": "test"})
}

func TestListenAndServe(t *testing.T) {
	s, _ := newTestServerWithFS()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx, "127.0.0.1", 19876)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil after shutdown, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server didn't shut down in time")
	}
}
