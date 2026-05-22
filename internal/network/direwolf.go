package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

type DirewolfClient struct {
	Host             string
	Port             int
	ReconnectInterval time.Duration
	Callsign         string
	SSID             int

	conn      net.Conn
	connected bool
	mu        sync.RWMutex
	kiss      *KISSProtocol
	logger    *slog.Logger

	OnFrame func([]byte)
}

func NewDirewolfClient(host string, port int, callsign string, ssid int, reconnectInterval time.Duration, logger *slog.Logger) *DirewolfClient {
	return &DirewolfClient{
		Host:              host,
		Port:              port,
		ReconnectInterval: reconnectInterval,
		Callsign:          callsign,
		SSID:              ssid,
		kiss:              NewKISSProtocol(0),
		logger:            logger,
	}
}

func (dc *DirewolfClient) IsConnected() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.connected
}

func (dc *DirewolfClient) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			dc.close()
			return
		default:
		}

		if err := dc.connect(ctx); err != nil {
			dc.logger.Warn("Failed to connect to Direwolf", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(dc.ReconnectInterval):
				continue
			}
		}

		dc.receiveLoop(ctx)
	}
}

func (dc *DirewolfClient) connect(ctx context.Context) error {
	addr := net.JoinHostPort(dc.Host, fmt.Sprintf("%d", dc.Port))
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	dc.mu.Lock()
	dc.conn = conn
	dc.connected = true
	dc.mu.Unlock()

	dc.kiss.Reset()
	dc.logger.Info("Connected to Direwolf", "addr", addr)
	return nil
}

func (dc *DirewolfClient) close() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.conn != nil {
		dc.conn.Close()
		dc.conn = nil
	}
	dc.connected = false
}

func (dc *DirewolfClient) receiveLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			dc.close()
			return
		default:
		}

		dc.mu.RLock()
		conn := dc.conn
		dc.mu.RUnlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			dc.logger.Warn("Direwolf connection lost", "error", err)
			dc.close()
			return
		}

		frames := dc.kiss.DecodeFrames(buf[:n])
		for _, frame := range frames {
			ax25Frame := DecodeAX25Frame(frame.Data)
			if ax25Frame != nil && ax25Frame.Control == 0x03 && ax25Frame.PID == 0xF0 && dc.OnFrame != nil {
				dc.OnFrame(ax25Frame.Info)
			}
		}
	}
}

func (dc *DirewolfClient) SendFrame(info []byte) error {
	dc.mu.RLock()
	conn := dc.conn
	dc.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	ax25Frame, err := CreateUIFrame(
		dc.fullCallsign(),
		"RFMP",
		info,
	)
	if err != nil {
		return err
	}

	ax25Bytes := ax25Frame.Encode()
	kissBytes := dc.kiss.EncodeData(ax25Bytes)

	_, err = conn.Write(kissBytes)
	return err
}

func (dc *DirewolfClient) fullCallsign() string {
	if dc.SSID == 0 {
		return dc.Callsign
	}
	return fmt.Sprintf("%s-%d", dc.Callsign, dc.SSID)
}
