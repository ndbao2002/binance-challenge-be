package binance_sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return &Client{
		baseURL:    base,
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{},
	}
}

func (c *Client) endpoint(path string, params map[string]string) (string, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range params {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	if c.token != "" {
		q.Set("token", c.token)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func (c *Client) Latest(ctx context.Context, symbol string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 200
	}
	ep, err := c.endpoint("/latest", map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var out []Snapshot
	if err := c.doJSON(ctx, http.MethodGet, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Stats(ctx context.Context) (Stats, error) {
	ep, err := c.endpoint("/stats", nil)
	if err != nil {
		return Stats{}, err
	}

	var out Stats
	if err := c.doJSON(ctx, http.MethodGet, ep, &out); err != nil {
		return Stats{}, err
	}
	return out, nil
}
