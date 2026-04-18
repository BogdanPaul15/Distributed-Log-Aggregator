package kafka

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"log-consumer/internal/model"

	"github.com/segmentio/kafka-go"
)

// MockReader implements MessageReader
type MockReader struct {
	messages     []kafka.Message
	currentIndex int
	mu           sync.Mutex
}

func (m *MockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate context cancellation if out of messages
	if m.currentIndex >= len(m.messages) {
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}

	msg := m.messages[m.currentIndex]
	m.currentIndex++
	return msg, nil
}

func (m *MockReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return nil
}

func (m *MockReader) Stats() kafka.ReaderStats {
	return kafka.ReaderStats{}
}

func (m *MockReader) Close() error {
	return nil
}

// MockStorage implements LogStorage
type MockStorage struct {
	indexedBatches int
	totalIndexed   int
	mu             sync.Mutex
}

func (m *MockStorage) IndexBatch(ctx context.Context, logs []model.LogEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexedBatches++
	m.totalIndexed += len(logs)
	return nil
}

func TestConsumer_Start_BatchProcessing(t *testing.T) {
	event := model.LogEvent{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     model.INFO,
		Service:   "test-service",
		Message:   "Test log",
	}
	val, _ := json.Marshal(event)

	mockReader := &MockReader{
		messages: []kafka.Message{
			{Value: val},
			{Value: val},
			{Value: val},
		},
	}
	mockStorage := &MockStorage{}

	consumer := NewConsumerWithDeps(mockReader, mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// Start blocks until context is cancelled
		_ = consumer.Start(ctx)
	}()

	// Wait enough time for consumer to fetch the 3 messages and flush due to flushInterval
	time.Sleep(1200 * time.Millisecond)
	cancel()
	wg.Wait()

	mockStorage.mu.Lock()
	defer mockStorage.mu.Unlock()

	if mockStorage.totalIndexed != 3 {
		t.Errorf("Expected 3 messages to be indexed, got %d", mockStorage.totalIndexed)
	}

	// Should have been indexed in a single batch due to interval flush
	if mockStorage.indexedBatches != 1 {
		t.Errorf("Expected 1 batch to be indexed, got %d", mockStorage.indexedBatches)
	}
}
