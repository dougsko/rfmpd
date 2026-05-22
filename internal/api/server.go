package api

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	mux         *http.ServeMux
	daemon      DaemonInterface
	logger      *slog.Logger
	upgrader    websocket.Upgrader
	wsClients   []*websocket.Conn
	wsMu        sync.Mutex
	corsOrigins []string
	webFS       fs.FS
	Version     string
}

type DaemonInterface interface {
	SendMessage(channel, body string, replyTo *string, author *string) (map[string]interface{}, error)
	GetStats() (map[string]interface{}, error)
	GetConfig() (string, int, string)
	SetCallsign(callsign string, ssid int) error
	GetDB() interface{}
	IsConnected() bool
}

type DatabaseInterface interface {
	GetMessageForAPI(id string) (map[string]interface{}, error)
	GetRecentMessagesForAPI(limit int, channel, fromNode *string) ([]map[string]interface{}, error)
	GetMessageCount() (int, error)
	GetActiveNodes(sinceSec int64) ([]map[string]interface{}, error)
	GetChannels() ([]map[string]interface{}, error)
	CreateChannel(name string) error
	DeleteChannel(name string) error
}

func NewServer(daemon DaemonInterface, corsOrigins []string, webFS fs.FS, logger *slog.Logger, version string) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		daemon:      daemon,
		logger:      logger,
		corsOrigins: corsOrigins,
		webFS:       webFS,
		Version:     version,
	}
	s.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			// Allow same-host connections (embedded UI served by this daemon)
			if r.Header.Get("Host") != "" && origin == "http://"+r.Host {
				return true
			}
			for _, o := range s.corsOrigins {
				if o == "*" || o == origin {
					return true
				}
			}
			return false
		},
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

func (s *Server) BroadcastMessage(data map[string]interface{}) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	msg := map[string]interface{}{
		"type": "message",
		"data": data,
	}

	var disconnected []int
	for i, conn := range s.wsClients {
		if err := conn.WriteJSON(msg); err != nil {
			disconnected = append(disconnected, i)
		}
	}

	for i := len(disconnected) - 1; i >= 0; i-- {
		idx := disconnected[i]
		s.wsClients[idx].Close()
		s.wsClients = append(s.wsClients[:idx], s.wsClients[idx+1:]...)
	}
}

func (s *Server) registerRoutes() {
	// Static web UI
	staticFS, _ := fs.Sub(s.webFS, "static")

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			data, _ := fs.ReadFile(s.webFS, "index.html")
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	})
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(s.webFS, "static/images/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	})

	// API routes
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/messages", s.handleMessages)
	s.mux.HandleFunc("/messages/", s.handleMessageByID)
	s.mux.HandleFunc("/nodes", s.handleNodes)
	s.mux.HandleFunc("/channels", s.handleChannels)
	s.mux.HandleFunc("/channels/", s.handleChannelByName)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/config/callsign", s.handleCallsign)
	s.mux.HandleFunc("/stream", s.handleStream)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowed := false
			for _, o := range s.corsOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				isWildcard := false
				for _, o := range s.corsOrigins {
					if o == "*" {
						isWildcard = true
						break
					}
				}
				if !isWildcard {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ListenAndServe(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	s.logger.Info("API server starting", "addr", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func pathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
