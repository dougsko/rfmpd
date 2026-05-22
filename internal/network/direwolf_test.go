package network

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"
)

func startMockDirewolf(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

func TestNewDirewolfClient(t *testing.T) {
	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", 8001, "N0CALL", 0, 5*time.Second, logger)
	if dc.Host != "127.0.0.1" {
		t.Errorf("host: %s", dc.Host)
	}
	if dc.Port != 8001 {
		t.Errorf("port: %d", dc.Port)
	}
	if dc.IsConnected() {
		t.Error("should not be connected initially")
	}
}

func TestDirewolfClient_ConnectAndDisconnect(t *testing.T) {
	ln, port := startMockDirewolf(t)
	defer ln.Close()

	// Accept connections in background
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Keep connection open until test ends
		buf := make([]byte, 1024)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", port, "N0CALL", 0, 100*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := dc.connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !dc.IsConnected() {
		t.Error("expected connected after connect")
	}

	dc.close()
	if dc.IsConnected() {
		t.Error("expected disconnected after close")
	}
}

func TestDirewolfClient_SendFrame(t *testing.T) {
	ln, port := startMockDirewolf(t)
	defer ln.Close()

	var received []byte
	var mu sync.Mutex
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		mu.Lock()
		received = buf[:n]
		mu.Unlock()
	}()

	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", port, "N0CALL", 0, 100*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dc.connect(ctx)
	err := dc.SendFrame([]byte("test payload"))
	if err != nil {
		t.Fatalf("SendFrame: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if len(received) == 0 {
		t.Fatal("expected data sent to mock server")
	}
	mu.Unlock()
}

func TestDirewolfClient_SendFrame_NotConnected(t *testing.T) {
	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", 8001, "N0CALL", 0, 100*time.Millisecond, logger)

	err := dc.SendFrame([]byte("test"))
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestDirewolfClient_ReceiveFrame(t *testing.T) {
	ln, port := startMockDirewolf(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a KISS-wrapped AX.25 UI frame
		frame, _ := CreateUIFrame("W1AW", "RFMP", []byte("hello from radio"))
		ax25Bytes := frame.Encode()
		kp := NewKISSProtocol(0)
		kissBytes := kp.EncodeData(ax25Bytes)
		conn.Write(kissBytes)

		// Keep connection alive
		time.Sleep(500 * time.Millisecond)
	}()

	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", port, "N0CALL", 0, 100*time.Millisecond, logger)

	var receivedPayload []byte
	var mu sync.Mutex
	dc.OnFrame = func(data []byte) {
		mu.Lock()
		receivedPayload = data
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dc.connect(ctx)

	// Run receive loop briefly
	go dc.receiveLoop(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if string(receivedPayload) != "hello from radio" {
		t.Fatalf("expected 'hello from radio', got %q", string(receivedPayload))
	}
}

func TestDirewolfClient_Run_ConnectFailure(t *testing.T) {
	logger := slog.Default()
	// Port that nobody is listening on
	dc := NewDirewolfClient("127.0.0.1", 19999, "N0CALL", 0, 50*time.Millisecond, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	dc.Run(ctx)
	// Should return without panic after context expires
}

func TestDirewolfClient_FullCallsign(t *testing.T) {
	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", 8001, "W1AW", 5, 5*time.Second, logger)
	if dc.fullCallsign() != "W1AW-5" {
		t.Errorf("expected W1AW-5, got %s", dc.fullCallsign())
	}

	dc2 := NewDirewolfClient("127.0.0.1", 8001, "N0CALL", 0, 5*time.Second, logger)
	if dc2.fullCallsign() != "N0CALL" {
		t.Errorf("expected N0CALL, got %s", dc2.fullCallsign())
	}
}

func TestDirewolfClient_Run_ConnectThenCancel(t *testing.T) {
	ln, port := startMockDirewolf(t)
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			defer conn.Close()
			time.Sleep(1 * time.Second)
		}
	}()

	logger := slog.Default()
	dc := NewDirewolfClient("127.0.0.1", port, "N0CALL", 0, 100*time.Millisecond, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	dc.Run(ctx)
}
