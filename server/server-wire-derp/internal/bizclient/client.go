package bizclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type DerpNode struct {
	RegionID          string          `json:"regionId"`
	NodeID            string          `json:"nodeId"`
	Name              string          `json:"name,omitempty"`
	Host              string          `json:"host"`
	Port              int             `json:"port,omitempty"`
	Enabled           *bool           `json:"enabled,omitempty"`
	Healthy           *bool           `json:"healthy,omitempty"`
	Priority          int             `json:"priority,omitempty"`
	TicketKeyRotation TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type Heartbeat struct {
	Healthy           bool            `json:"healthy"`
	TicketKeyRotation TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type TicketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}},
	}
}

func (c *Client) Enabled() bool {
	return c.baseURL != "" && c.token != ""
}

func (c *Client) UpsertDerpNode(ctx context.Context, node DerpNode) error {
	return c.doJSON(ctx, http.MethodPut, "/internal/wire/admin/derp-nodes", node)
}

func (c *Client) HeartbeatDerpNode(ctx context.Context, regionID, nodeID string, healthy bool, ticket TicketKeyStatus) error {
	path := fmt.Sprintf("/internal/wire/admin/derp-nodes/%s/%s/heartbeat", regionID, nodeID)
	return c.doJSON(ctx, http.MethodPost, path, Heartbeat{Healthy: healthy, TicketKeyRotation: ticket})
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any) error {
	payload, err := json.Marshal(in)
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
