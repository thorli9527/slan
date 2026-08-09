package bizclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type PunchNode struct {
	NodeID   string `json:"nodeId"`
	Name     string `json:"name,omitempty"`
	Host     string `json:"host"`
	UDPPort  int    `json:"udpPort"`
	Priority int    `json:"priority,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Healthy  *bool  `json:"healthy,omitempty"`
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}},
	}
}

func (c *Client) Enabled() bool {
	parsed, err := url.Parse(c.baseURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && c.token != ""
}

func (c *Client) UpsertPunchNode(ctx context.Context, node PunchNode) error {
	return c.doJSON(ctx, http.MethodPut, "/internal/wire/admin/punch-nodes", node)
}

func (c *Client) HeartbeatPunchNode(ctx context.Context, nodeID string, healthy bool) error {
	path := "/internal/wire/admin/punch-nodes/" + url.PathEscape(strings.TrimSpace(nodeID)) + "/heartbeat"
	return c.doJSON(ctx, http.MethodPost, path, map[string]bool{"healthy": healthy})
}

func (c *Client) doJSON(ctx context.Context, method, path string, input any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s status=%d", method, path, resp.StatusCode)
	}
	return nil
}
