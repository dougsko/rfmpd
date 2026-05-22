package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	ID        string  `json:"id"`
	FromNode  string  `json:"from_node"`
	Author    *string `json:"author"`
	Timestamp string  `json:"timestamp"`
	Channel   string  `json:"channel"`
	ReplyTo   *string `json:"reply_to"`
	Body      string  `json:"body"`
}

func SendMessage(apiPort int, channel, body string) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/messages", apiPort)
	payloadBytes, err := json.Marshal(map[string]string{"channel": channel, "body": body})
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("send message failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if id, ok := result["id"].(string); ok && id != "" {
		return id, nil
	}
	return "", fmt.Errorf("no message ID in response")
}

func GetMessages(apiPort int, channel string, limit int) ([]Message, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/messages?channel=%s&limit=%d", apiPort, channel, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func GetMessageIDs(apiPort int, channel string) (map[string]bool, error) {
	msgs, err := GetMessages(apiPort, channel, 1000)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(msgs))
	for _, m := range msgs {
		ids[m.ID] = true
	}
	return ids, nil
}

func WaitHealthy(apiPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", apiPort)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("node on port %d not healthy after %v", apiPort, timeout)
}

func WaitIDConvergence(nodes []*Node, channel string, expectedIDs map[string]bool, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	deadline := start.Add(timeout)

	checkConverged := func() bool {
		for _, node := range nodes {
			if !node.IsRunning() {
				continue
			}
			ids, err := GetMessageIDs(node.APIPort, channel)
			if err != nil {
				return false
			}
			for id := range expectedIDs {
				if !ids[id] {
					return false
				}
			}
		}
		return true
	}

	for time.Now().Before(deadline) {
		if checkConverged() {
			return time.Since(start), nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final check to avoid race where timeout fires just as nodes finish converging
	if checkConverged() {
		return time.Since(start), nil
	}

	var details []string
	for _, node := range nodes {
		if !node.IsRunning() {
			continue
		}
		ids, err := GetMessageIDs(node.APIPort, channel)
		if err != nil {
			details = append(details, fmt.Sprintf("%s: error", node.Callsign))
		} else {
			missing := 0
			for id := range expectedIDs {
				if !ids[id] {
					missing++
				}
			}
			details = append(details, fmt.Sprintf("%s: has %d, missing %d", node.Callsign, len(ids), missing))
		}
	}
	return time.Since(start), fmt.Errorf("ID convergence timeout: %s", strings.Join(details, ", "))
}
