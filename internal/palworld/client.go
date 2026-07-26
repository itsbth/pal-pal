package palworld

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

const maxResponseSize = 16 << 20

type Client struct {
	root       *url.URL
	password   string
	httpClient *http.Client
}

func NewClient(root, password string) (*Client, error) {
	parsed, err := url.Parse(root)
	if err != nil {
		return nil, fmt.Errorf("parse API_ROOT: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("API_ROOT must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("API_ROOT must include a host")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/v1/api"
	}

	return &Client{
		root:     parsed,
		password: password,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) Info(ctx context.Context) (domain.ServerInfo, error) {
	var info domain.ServerInfo
	err := c.do(ctx, http.MethodGet, "/info", nil, &info)
	return info, err
}

func (c *Client) Players(ctx context.Context) ([]domain.Player, error) {
	var response struct {
		Players []domain.Player `json:"players"`
	}
	err := c.do(ctx, http.MethodGet, "/players", nil, &response)
	return response.Players, err
}

func (c *Client) Metrics(ctx context.Context) (domain.Metrics, error) {
	var metrics domain.Metrics
	err := c.do(ctx, http.MethodGet, "/metrics", nil, &metrics)
	return metrics, err
}

func (c *Client) GameData(ctx context.Context) (domain.GameData, error) {
	var gameData domain.GameData
	err := c.do(ctx, http.MethodGet, "/game-data", nil, &gameData)
	return gameData, err
}

func (c *Client) Settings(ctx context.Context) (map[string]any, error) {
	settings := make(map[string]any)
	err := c.do(ctx, http.MethodGet, "/settings", nil, &settings)
	return settings, err
}

func (c *Client) Kick(ctx context.Context, userID, message string) error {
	return c.do(ctx, http.MethodPost, "/kick", map[string]string{
		"userid":  userID,
		"message": message,
	}, nil)
}

func (c *Client) Ban(ctx context.Context, userID, message string) error {
	return c.do(ctx, http.MethodPost, "/ban", map[string]string{
		"userid":  userID,
		"message": message,
	}, nil)
}

func (c *Client) Unban(ctx context.Context, userID string) error {
	return c.do(ctx, http.MethodPost, "/unban", map[string]string{
		"userid": userID,
	}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any, destination any) error {
	target := *c.root
	target.Path = strings.TrimRight(c.root.Path, "/") + "/" + strings.TrimLeft(path, "/")
	target.RawPath = ""

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s request: %w", path, err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, target.String(), requestBody)
	if err != nil {
		return fmt.Errorf("create %s request: %w", path, err)
	}
	request.SetBasicAuth("admin", c.password)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, maxResponseSize)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(limited, 2048))
		return fmt.Errorf("%s %s: upstream returned %s: %s", method, path, response.Status, strings.TrimSpace(string(message)))
	}
	if destination == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(destination); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}
