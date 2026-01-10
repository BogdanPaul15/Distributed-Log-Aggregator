package storage

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"log-consumer/internal/model"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	indexingErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "opensearch_indexing_errors_total",
		Help: "The total number of failed indexing attempts",
	})
	indexingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "opensearch_indexing_duration_seconds",
		Help:    "The duration of indexing requests to OpenSearch",
		Buckets: prometheus.DefBuckets,
	})
)

type OpenSearchClient struct {
	client *opensearch.Client
}

func NewOpenSearchClient(addresses []string) (*OpenSearchClient, error) {
	cfg := opensearch.Config{
		Addresses: addresses,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create opensearch client: %w", err)
	}

	return &OpenSearchClient{client: client}, nil
}

func (c *OpenSearchClient) IndexLog(ctx context.Context, event model.LogEvent) error {
	timer := prometheus.NewTimer(indexingDuration)
	defer timer.ObserveDuration()

	// Parse timestamp to generate index name
	// Assuming timestamp is in RFC3339 format
	t, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		log.Printf("Failed to parse timestamp %s: %v", event.Timestamp, err)
		// Fallback to current time if parsing fails
		t = time.Now()
	}

	indexName := fmt.Sprintf("app-logs-%s", t.Format("2006.01.02"))

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req := opensearchapi.IndexRequest{
		Index: indexName,
		Body:  strings.NewReader(string(body)),
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		indexingErrors.Inc()
		return fmt.Errorf("failed to execute index request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		indexingErrors.Inc()
		return fmt.Errorf("opensearch error: %s", res.String())
	}

	return nil
}

func (c *OpenSearchClient) IndexBatch(ctx context.Context, events []model.LogEvent) error {
	if len(events) == 0 {
		return nil
	}

	timer := prometheus.NewTimer(indexingDuration)
	defer timer.ObserveDuration()

	var buf strings.Builder

	for _, event := range events {
		t, err := time.Parse(time.RFC3339, event.Timestamp)
		if err != nil {
			t = time.Now()
		}
		indexName := fmt.Sprintf("app-logs-%s", t.Format("2006.01.02"))

		meta := fmt.Sprintf(`{ "index": { "_index": "%s" } }%s`, indexName, "\n")
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("Failed to marshal event for bulk index: %v", err)
			continue
		}

		buf.WriteString(meta)
		buf.Write(data)
		buf.WriteString("\n")
	}

	req := opensearchapi.BulkRequest{
		Body: strings.NewReader(buf.String()),
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		indexingErrors.Inc()
		return fmt.Errorf("failed to execute bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		indexingErrors.Inc()
		return fmt.Errorf("opensearch bulk error: %s", res.String())
	}

	return nil
}
