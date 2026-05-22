package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type Node struct {
	ID         int
	Callsign   string
	APIPort    int
	BrokerPort int
	ConfigPath string
	DBPath     string
	TmpDir     string

	cmd     *exec.Cmd
	verbose bool
}

type NodeOption func(*nodeOpts)

type nodeOpts struct {
	fragmentThreshold int
}

func WithFragmentThreshold(t int) NodeOption {
	return func(o *nodeOpts) { o.fragmentThreshold = t }
}

func StartNode(id int, callsign string, apiPort, brokerPort int, verbose bool, rfmpdBin string, opts ...NodeOption) (*Node, error) {
	o := &nodeOpts{fragmentThreshold: 256}
	for _, opt := range opts {
		opt(o)
	}
	fragmentThreshold := o.fragmentThreshold

	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("rf-sim-node%d-*", id))
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(tmpDir, "messages.db")
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := fmt.Sprintf(`node:
  callsign: "%s"
  ssid: %d
network:
  direwolf_host: "127.0.0.1"
  direwolf_port: %d
  reconnect_interval: 2
  offline_mode: false
protocol:
  fragment_threshold: %d
timing:
  base_delay: 0.05
  jitter: 0.05
sync:
  sync_interval: 3
storage:
  database_path: "%s"
api:
  host: "127.0.0.1"
  port: %d
  cors_origins:
    - "http://localhost:3000"
logging:
  level: "INFO"
`, callsign, id, brokerPort, fragmentThreshold, dbPath, apiPort)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	node := &Node{
		ID:         id,
		Callsign:   callsign,
		APIPort:    apiPort,
		BrokerPort: brokerPort,
		ConfigPath: configPath,
		DBPath:     dbPath,
		TmpDir:     tmpDir,
		verbose:    verbose,
	}

	if err := node.start(rfmpdBin); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return node, nil
}

func (n *Node) start(rfmpdBin string) error {
	n.cmd = exec.Command(rfmpdBin, "-c", n.ConfigPath, "--sim", "--sim-port", fmt.Sprintf("%d", n.BrokerPort))

	if n.verbose {
		n.cmd.Stdout = os.Stdout
		n.cmd.Stderr = os.Stderr
	} else {
		n.cmd.Stdout = io.Discard
		n.cmd.Stderr = io.Discard
	}

	if err := n.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start node %d: %w", n.ID, err)
	}

	if err := WaitHealthy(n.APIPort, 10*time.Second); err != nil {
		n.cmd.Process.Kill()
		return fmt.Errorf("node %d failed to become healthy: %w", n.ID, err)
	}

	return nil
}

func (n *Node) Stop() error {
	if n.cmd == nil || n.cmd.Process == nil {
		return nil
	}
	n.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- n.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		n.cmd.Process.Kill()
	}
	n.cmd = nil
	return nil
}

func (n *Node) Kill() error {
	if n.cmd == nil || n.cmd.Process == nil {
		return nil
	}
	n.cmd.Process.Kill()
	n.cmd.Wait()
	n.cmd = nil
	return nil
}

func (n *Node) Restart(rfmpdBin string) error {
	return n.start(rfmpdBin)
}

func (n *Node) Cleanup() {
	n.Stop()
	os.RemoveAll(n.TmpDir)
}

func (n *Node) IsRunning() bool {
	if n.cmd == nil || n.cmd.Process == nil {
		return false
	}
	err := n.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}
