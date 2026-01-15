package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"log-generator/internal/model"
)

type HTTPConfig struct {
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}

type HTTPStorage struct {
	url    string
	client *http.Client
}

func NewHTTPStorage(cfg HTTPConfig) *HTTPStorage {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 1000
	t.MaxIdleConnsPerHost = 100
	t.DisableKeepAlives = true

	return &HTTPStorage{
		url: cfg.URL,
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: t,
		},
	}
}

func (hs *HTTPStorage) Store(ctx context.Context, event model.LogEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := hs.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned error status: %s", resp.Status)
	}

	return nil
}

func (hs *HTTPStorage) StoreBatch(ctx context.Context, events []model.LogEvent) error {
	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := hs.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned error status: %s", resp.Status)
	}

	return nil
}

func (hs *HTTPStorage) Close() error {
	hs.client.CloseIdleConnections()
	return nil
}
