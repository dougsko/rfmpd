package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type Channel struct {
	Name         string `json:"name"`
	MessageCount int    `json:"message_count"`
}

type Message struct {
	ID        string  `json:"id"`
	FromNode  string  `json:"from_node"`
	Author    *string `json:"author"`
	Timestamp string  `json:"timestamp"`
	Channel   string  `json:"channel"`
	ReplyTo   *string `json:"reply_to"`
	Body      string  `json:"body"`
}

type Status struct {
	Version   string `json:"version"`
	Connected bool   `json:"connected_to_direwolf"`
	Callsign  string `json:"node_callsign"`
	Stats     struct {
		MessageCount int `json:"message_count"`
		ActiveNodes  int `json:"active_nodes"`
	} `json:"stats"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) GetChannels() ([]Channel, error) {
	resp, err := c.http.Get(c.baseURL + "/channels")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get channels: %d %s", resp.StatusCode, body)
	}

	var channels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) GetMessages(channel string, limit int) ([]Message, error) {
	url := fmt.Sprintf("%s/messages?channel=%s&limit=%d", c.baseURL, channel, limit)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages: %d %s", resp.StatusCode, body)
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (c *Client) SendMessage(channel, body string, author *string, replyTo *string) (*Message, error) {
	payload := map[string]interface{}{
		"channel": channel,
		"body":    body,
	}
	if author != nil {
		payload["author"] = *author
	}
	if replyTo != nil {
		payload["reply_to"] = *replyTo
	}

	data, _ := json.Marshal(payload)
	resp, err := c.http.Post(c.baseURL+"/messages", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send message: %d %s", resp.StatusCode, respBody)
	}

	var msg Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *Client) CreateChannel(name string) error {
	data, _ := json.Marshal(map[string]string{"name": name})
	resp, err := c.http.Post(c.baseURL+"/channels", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}
	return nil
}

func (c *Client) DeleteChannel(name string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/channels/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}
	return nil
}

func (c *Client) GetStatus() (*Status, error) {
	resp, err := c.http.Get(c.baseURL + "/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get status: %d %s", resp.StatusCode, body)
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

type ConfigData struct {
	Node struct {
		Callsign string `json:"callsign"`
		SSID     int    `json:"ssid"`
	} `json:"node"`
	Network struct {
		DirewolfHost      string `json:"direwolf_host"`
		DirewolfPort      int    `json:"direwolf_port"`
		ReconnectInterval int    `json:"reconnect_interval"`
		OfflineMode       bool   `json:"offline_mode"`
	} `json:"network"`
	Timing struct {
		BaseDelay float64 `json:"base_delay"`
		Jitter    float64 `json:"jitter"`
	} `json:"timing"`
	Sync struct {
		SyncInterval int `json:"sync_interval"`
	} `json:"sync"`
	Logging struct {
		Level string `json:"level"`
	} `json:"logging"`
}

func (c *Client) GetConfig() (*ConfigData, error) {
	resp, err := c.http.Get(c.baseURL + "/config")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get config: %d %s", resp.StatusCode, body)
	}

	var cfg ConfigData
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Client) SaveConfig(cfg *ConfigData) error {
	data, _ := json.Marshal(cfg)
	req, err := http.NewRequest("PUT", c.baseURL+"/config", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}
	return nil
}

func (c *Client) Shutdown() error {
	resp, err := c.http.Post(c.baseURL+"/shutdown", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
