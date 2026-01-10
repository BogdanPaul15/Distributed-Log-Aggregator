package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"time"

	"log-generator/internal/model"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Gauge: How many requests are currently waiting for a response?
	httpClientInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_client_in_flight_requests",
		Help: "Number of HTTP requests currently in progress",
	})

	// Counter: Are we creating new connections or reusing old ones?
	httpClientConnReused = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_conn_reused_total",
		Help: "Total number of connections obtained, labeled by whether they were reused",
	}, []string{"reused"}) // Label: "true" or "false"

	// Histogram: How long do we wait to get a connection? (Detects pool exhaustion)
	httpClientGetConnDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_client_get_conn_duration_seconds",
		Help:    "Time spent waiting for a free connection from the pool",
		Buckets: []float64{0.001, 0.005, 0.01, 0.1, 1}, // 1ms to 1s
	})
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

	var getConnStart time.Time

	trace := &httptrace.ClientTrace{
		// Hook 1: We started asking for a connection (from pool or new)
		GetConn: func(hostPort string) {
			getConnStart = time.Now()
		},
		// Hook 2: We got a connection!
		GotConn: func(info httptrace.GotConnInfo) {
			// Measure how long we waited (Blocked by MaxConnsPerHost?)
			httpClientGetConnDuration.Observe(time.Since(getConnStart).Seconds())

			// Record if it was reused (Idle) or new (Dialed)
			httpClientConnReused.WithLabelValues(fmt.Sprintf("%t", info.Reused)).Inc()
		},
	}

	ctx = httptrace.WithClientTrace(ctx, trace)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 5. Track In-Flight Requests
	httpClientInFlight.Inc()
	defer httpClientInFlight.Dec()

	resp, err := hs.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (hs *HTTPStorage) StoreBatch(ctx context.Context, events []model.LogEvent) error {
	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	var getConnStart time.Time

	trace := &httptrace.ClientTrace{
		// Hook 1: We started asking for a connection (from pool or new)
		GetConn: func(hostPort string) {
			getConnStart = time.Now()
		},
		// Hook 2: We got a connection!
		GotConn: func(info httptrace.GotConnInfo) {
			// Measure how long we waited (Blocked by MaxConnsPerHost?)
			httpClientGetConnDuration.Observe(time.Since(getConnStart).Seconds())

			// Record if it was reused (Idle) or new (Dialed)
			httpClientConnReused.WithLabelValues(fmt.Sprintf("%t", info.Reused)).Inc()
		},
	}

	ctx = httptrace.WithClientTrace(ctx, trace)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 5. Track In-Flight Requests
	httpClientInFlight.Inc()
	defer httpClientInFlight.Dec()

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
