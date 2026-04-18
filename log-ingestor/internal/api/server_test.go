package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log-ingestor/internal/model"
)

// MockProducer implements kafka.ProducerInterface for testing
type MockProducer struct {
	produceCalled      int
	produceBatchCalled int
	lastBatch          []model.LogEvent
	returnError        error
}

func (m *MockProducer) Produce(ctx context.Context, event model.LogEvent) error {
	m.produceCalled++
	return m.returnError
}

func (m *MockProducer) ProduceBatch(ctx context.Context, events []model.LogEvent) error {
	m.produceBatchCalled++
	m.lastBatch = events
	return m.returnError
}

func (m *MockProducer) Close() error {
	return nil
}

func TestServer_HandleLogs_ValidBatch(t *testing.T) {
	mockProducer := &MockProducer{}
	server := NewServer(mockProducer)

	events := []model.LogEvent{
		{
			Timestamp: "2023-01-01T12:00:00Z",
			Level:     model.INFO,
			Service:   "test-service",
			Message:   "Test log",
		},
	}
	body, _ := json.Marshal(events)
	req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, res.StatusCode)
	}

	if mockProducer.produceBatchCalled != 1 {
		t.Errorf("Expected ProduceBatch to be called 1 time, got %d", mockProducer.produceBatchCalled)
	}

	if len(mockProducer.lastBatch) != 1 || mockProducer.lastBatch[0].Message != "Test log" {
		t.Errorf("Expected last batch to contain the test event, got %+v", mockProducer.lastBatch)
	}
}

func TestServer_HandleLogs_InvalidMethod(t *testing.T) {
	mockProducer := &MockProducer{}
	server := NewServer(mockProducer)

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, res.StatusCode)
	}
}

func TestServer_HandleLogs_InvalidJSON(t *testing.T) {
	mockProducer := &MockProducer{}
	server := NewServer(mockProducer)

	req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}
