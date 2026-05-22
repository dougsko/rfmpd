package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/dprostko/rfmp-go/internal/api"
	"github.com/dprostko/rfmp-go/internal/config"
	"github.com/dprostko/rfmp-go/internal/daemon"
)

var version = "dev"

//go:embed web
var embeddedWeb embed.FS

func main() {
	configPath := flag.String("c", "", "path to config file")
	verbose := flag.Bool("v", false, "verbose logging")
	showVersion := flag.Bool("version", false, "print version and exit")
	sim := flag.Bool("sim", false, "connect to RF simulator broker instead of Direwolf")
	simPort := flag.Int("sim-port", 8055, "RF simulator broker port")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *sim {
		cfg.Network.DirewolfHost = "127.0.0.1"
		cfg.Network.DirewolfPort = *simPort
		cfg.Network.OfflineMode = false
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	} else if strings.EqualFold(cfg.Logging.Level, "DEBUG") {
		level = slog.LevelDebug
	} else if strings.EqualFold(cfg.Logging.Level, "WARNING") || strings.EqualFold(cfg.Logging.Level, "WARN") {
		level = slog.LevelWarn
	} else if strings.EqualFold(cfg.Logging.Level, "ERROR") {
		level = slog.LevelError
	}

	// Setup logging with optional file output
	var logWriter io.Writer = os.Stderr
	if cfg.Logging.File != "" {
		logDir := filepath.Dir(cfg.Logging.File)
		if logDir != "" && logDir != "." {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "FATAL: cannot create log directory %s: %v\n", logDir, err)
				os.Exit(1)
			}
		}
		logFile := &rotatingWriter{
			path:        cfg.Logging.File,
			maxSize:     int64(cfg.Logging.MaxSize),
			backupCount: cfg.Logging.BackupCount,
		}
		logWriter = io.MultiWriter(os.Stderr, logFile)
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: level}))

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.Storage.DatabasePath)
	if dbDir != "" && dbDir != "." {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: cannot create database directory %s: %v\n", dbDir, err)
			os.Exit(1)
		}
	}

	logger.Info("Starting RFMP daemon",
		"callsign", cfg.Node.Callsign,
		"ssid", cfg.Node.SSID,
		"offline", cfg.Network.OfflineMode,
	)

	d, err := daemon.New(cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize daemon: %v\n", err)
		os.Exit(1)
	}
	d.ConfigPath = *configPath

	webContent, _ := fs.Sub(embeddedWeb, "web")
	server := api.NewServer(d, cfg.API.CORSOrigins, webContent, logger, version)
	d.SetAPIServer(server)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	// Start API server in background
	go func() {
		if err := server.ListenAndServe(ctx, cfg.API.Host, cfg.API.Port); err != nil {
			logger.Error("API server error", "error", err)
			cancel()
		}
	}()

	// Run daemon (blocks until context cancelled)
	if err := d.Run(ctx); err != nil {
		logger.Error("Daemon error", "error", err)
		os.Exit(1)
	}
}

// rotatingWriter implements io.Writer with file size rotation.
type rotatingWriter struct {
	path        string
	maxSize     int64
	backupCount int
	mu          sync.Mutex
	file        *os.File
	size        int64
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		w.rotate()
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) openFile() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) rotate() {
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}

	// Shift existing backups
	for i := w.backupCount - 1; i > 0; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		os.Rename(src, dst)
	}
	if w.backupCount > 0 {
		os.Rename(w.path, fmt.Sprintf("%s.1", w.path))
	} else {
		os.Remove(w.path)
	}

	w.openFile()
}
