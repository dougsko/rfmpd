package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dougsko/rfmpd/internal/config"
)

var channelRegex = regexp.MustCompile(`^[a-z0-9_-]{1,20}$`)
var callsignRegex = regexp.MustCompile(`^[A-Z0-9]{1,6}$`)

func (s *Server) getDB() DatabaseInterface {
	return s.daemon.GetDB().(DatabaseInterface)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "healthy"})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.getMessages(w, r)
	case "POST":
		s.postMessage(w, r)
	default:
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
	}
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 && v <= 1000 {
			limit = v
		}
	}

	var channel, fromNode *string
	if c := r.URL.Query().Get("channel"); c != "" {
		channel = &c
	}
	if f := r.URL.Query().Get("from_node"); f != "" {
		fromNode = &f
	}

	messages, err := s.getDB().GetRecentMessagesForAPI(limit, channel, fromNode)
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	if messages == nil {
		messages = []map[string]interface{}{}
	}
	writeJSON(w, 200, messages)
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channel string  `json:"channel"`
		Body    string  `json:"body"`
		ReplyTo *string `json:"reply_to"`
		Author  *string `json:"author"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 422, map[string]string{"detail": "invalid request body"})
		return
	}

	if req.Channel == "" || req.Body == "" {
		writeJSON(w, 422, map[string]string{"detail": "channel and body are required"})
		return
	}

	if !channelRegex.MatchString(req.Channel) {
		writeJSON(w, 422, map[string]string{"detail": "channel must be 1-20 lowercase alphanumeric characters, hyphens, or underscores"})
		return
	}

	if len(req.Body) > 10000 {
		writeJSON(w, 422, map[string]string{"detail": "body must be 10000 characters or fewer"})
		return
	}

	msg, err := s.daemon.SendMessage(req.Channel, req.Body, req.ReplyTo, req.Author)
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	writeJSON(w, 201, msg)
}

func (s *Server) handleMessageByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
		return
	}
	id := pathParam(r.URL.Path, "/messages/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	msg, err := s.getDB().GetMessageForAPI(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}
	if msg == nil {
		writeJSON(w, 404, map[string]string{"detail": "Message not found"})
		return
	}
	writeJSON(w, 200, msg)
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
		return
	}

	hours := 24
	if h := r.URL.Query().Get("active_hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v >= 1 && v <= 8760 {
			hours = v
		}
	}

	nodes, err := s.getDB().GetActiveNodes(int64(hours) * 3600)
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	// Format timestamps
	result := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		formatted := map[string]interface{}{
			"callsign":      node["callsign"],
			"first_seen":    formatUnix(node["first_seen"]),
			"last_seen":     formatUnix(node["last_seen"]),
			"message_count": node["message_count"],
			"sync_count":    node["sync_count"],
			"req_count":     node["req_count"],
		}
		if ls, ok := node["last_sync"].(*int64); ok && ls != nil {
			formatted["last_sync"] = formatUnix(ls)
		} else {
			formatted["last_sync"] = nil
		}
		result = append(result, formatted)
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.getChannelsList(w, r)
	case "POST":
		s.createChannel(w, r)
	default:
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
	}
}

func (s *Server) getChannelsList(w http.ResponseWriter, r *http.Request) {
	channels, err := s.getDB().GetChannels()
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	result := make([]map[string]interface{}, 0, len(channels))
	for _, ch := range channels {
		result = append(result, map[string]interface{}{
			"name":          ch["name"],
			"first_message": formatUnix(ch["first_message"]),
			"last_message":  formatUnix(ch["last_message"]),
			"message_count": ch["message_count"],
			"unique_nodes":  ch["unique_nodes"],
		})
	}
	writeJSON(w, 200, result)
}

func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 422, map[string]string{"detail": "invalid request body"})
		return
	}

	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if !channelRegex.MatchString(req.Name) {
		writeJSON(w, 422, map[string]string{"detail": "channel name must be 1-20 lowercase alphanumeric characters, hyphens, or underscores"})
		return
	}

	if err := s.getDB().CreateChannel(req.Name); err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	writeJSON(w, 201, map[string]interface{}{
		"name":          req.Name,
		"message_count": 0,
	})
}

func (s *Server) handleChannelByName(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r.URL.Path, "/channels/")
	if name == "" {
		writeJSON(w, 400, map[string]string{"detail": "channel name required"})
		return
	}
	switch r.Method {
	case "DELETE":
		err := s.getDB().DeleteChannel(name)
		if err != nil {
			if err.Error() == "channel has messages" {
				writeJSON(w, 409, map[string]string{"detail": "cannot delete channel with messages"})
			} else if err.Error() == "channel not found" {
				writeJSON(w, 404, map[string]string{"detail": "channel not found"})
			} else {
				writeJSON(w, 500, map[string]string{"detail": err.Error()})
			}
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
		return
	}

	stats, err := s.daemon.GetStats()
	if err != nil {
		writeJSON(w, 500, map[string]string{"detail": err.Error()})
		return
	}

	_, _, fullCallsign := s.daemon.GetConfig()

	result := map[string]interface{}{
		"version":                s.Version,
		"uptime_seconds":         stats["uptime_seconds"],
		"connected_to_direwolf": s.daemon.IsConnected(),
		"node_callsign":         fullCallsign,
		"stats":                 stats,
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleCallsign(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		callsign, ssid, fullCallsign := s.daemon.GetConfig()
		writeJSON(w, 200, map[string]interface{}{
			"callsign":      callsign,
			"ssid":          ssid,
			"full_callsign": fullCallsign,
		})
	case "POST":
		var req struct {
			Callsign string `json:"callsign"`
			SSID     int    `json:"ssid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 422, map[string]string{"detail": "invalid request"})
			return
		}
		req.Callsign = strings.ToUpper(strings.TrimSpace(req.Callsign))
		if !callsignRegex.MatchString(req.Callsign) {
			writeJSON(w, 422, map[string]string{"detail": "callsign must be 1-6 uppercase alphanumeric characters"})
			return
		}
		if req.SSID < 0 || req.SSID > 15 {
			writeJSON(w, 422, map[string]string{"detail": "ssid must be between 0 and 15"})
			return
		}
		if err := s.daemon.SetCallsign(req.Callsign, req.SSID); err != nil {
			writeJSON(w, 500, map[string]string{"detail": err.Error()})
			return
		}
		fullCallsign := req.Callsign
		if req.SSID > 0 {
			fullCallsign = req.Callsign + "-" + strconv.Itoa(req.SSID)
		}
		writeJSON(w, 200, map[string]interface{}{
			"success":       true,
			"callsign":      req.Callsign,
			"ssid":          req.SSID,
			"full_callsign": fullCallsign,
		})
	default:
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.wsMu.Lock()
	s.wsClients = append(s.wsClients, conn)
	count := len(s.wsClients)
	s.wsMu.Unlock()

	s.logger.Info("WebSocket client connected", "clients", count)

	defer func() {
		s.wsMu.Lock()
		for i, c := range s.wsClients {
			if c == conn {
				s.wsClients = append(s.wsClients[:i], s.wsClients[i+1:]...)
				break
			}
		}
		s.wsMu.Unlock()
		conn.Close()
		s.logger.Info("WebSocket client disconnected")
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cfg := s.daemon.GetFullConfig()
		writeJSON(w, 200, cfg)
	case "PUT":
		existing := s.daemon.GetFullConfig()
		var incoming config.Config
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeJSON(w, 422, map[string]string{"detail": "invalid JSON: " + err.Error()})
			return
		}
		// Merge: only override fields the client sends (non-zero)
		merged := *existing
		if incoming.Node.Callsign != "" {
			merged.Node.Callsign = strings.ToUpper(strings.TrimSpace(incoming.Node.Callsign))
		}
		merged.Node.SSID = incoming.Node.SSID
		if incoming.Network.DirewolfHost != "" {
			merged.Network.DirewolfHost = incoming.Network.DirewolfHost
		}
		if incoming.Network.DirewolfPort > 0 {
			merged.Network.DirewolfPort = incoming.Network.DirewolfPort
		}
		merged.Network.OfflineMode = incoming.Network.OfflineMode
		if incoming.Sync.SyncInterval > 0 {
			merged.Sync.SyncInterval = incoming.Sync.SyncInterval
		}
		if incoming.Logging.Level != "" {
			merged.Logging.Level = incoming.Logging.Level
		}
		if merged.Node.Callsign == "" {
			writeJSON(w, 422, map[string]string{"detail": "callsign is required"})
			return
		}
		if err := s.daemon.SaveConfig(&merged); err != nil {
			writeJSON(w, 500, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "saved, restarting"})
		go func() {
			time.Sleep(500 * time.Millisecond)
			exec.Command("systemctl", "restart", "rfmpd").Start()
		}()
	default:
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
	}
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"detail": "method not allowed"})
		return
	}
	s.logger.Info("Shutdown requested via API")
	writeJSON(w, 200, map[string]string{"status": "shutting down"})
	go func() {
		time.Sleep(500 * time.Millisecond)
		exec.Command("shutdown", "-h", "now").Start()
	}()
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func formatUnix(v interface{}) string {
	switch val := v.(type) {
	case int64:
		return time.Unix(val, 0).Format(time.RFC3339)
	case *int64:
		if val == nil {
			return ""
		}
		return time.Unix(*val, 0).Format(time.RFC3339)
	case int:
		return time.Unix(int64(val), 0).Format(time.RFC3339)
	default:
		return ""
	}
}
