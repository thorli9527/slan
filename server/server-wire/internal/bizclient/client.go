package bizclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) PeerAuthz(ctx context.Context, peerID string) (model.PeerAuthzView, error) {
	var out model.PeerAuthzView
	if err := c.get(ctx, "/internal/wire/peers/"+peerID+"/authz", &out); err != nil {
		return model.PeerAuthzView{}, err
	}
	return out, nil
}

func (c *Client) PeerRuntimeConfig(ctx context.Context, peerID string) (model.PeerRuntimeConfigView, error) {
	var out model.PeerRuntimeConfigView
	if err := c.get(ctx, "/internal/wire/peers/"+peerID+"/runtime-config", &out); err != nil {
		return model.PeerRuntimeConfigView{}, err
	}
	return out, nil
}

func (c *Client) NetworkTopology(ctx context.Context, networkID string) (model.NetworkTopologyView, error) {
	var out model.NetworkTopologyView
	if err := c.get(ctx, "/internal/wire/networks/"+networkID+"/topology", &out); err != nil {
		return model.NetworkTopologyView{}, err
	}
	return out, nil
}

func (c *Client) DerpMap(ctx context.Context) (model.DerpMap, error) {
	var out model.DerpMap
	if err := c.get(ctx, "/internal/wire/derp-map", &out); err != nil {
		return model.DerpMap{}, err
	}
	return out, nil
}

func (c *Client) RelayNodes(ctx context.Context) ([]model.RelayNode, error) {
	var out struct {
		Items []model.RelayNode `json:"items"`
	}
	if err := c.get(ctx, "/internal/wire/admin/relay-nodes", &out); err != nil {
		return nil, err
	}
	return append([]model.RelayNode(nil), out.Items...), nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("X-Slan-Internal-Token", c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server-biz internal %s returned %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
