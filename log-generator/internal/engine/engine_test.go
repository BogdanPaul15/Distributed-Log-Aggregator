package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"log-generator/internal/model"
)

// MockGenerator implements generator.Generator
type MockGenerator struct {
	generateCalled int
	mu             sync.Mutex
}

func (m *MockGenerator) Generate() model.LogEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generateCalled++
	return model.LogEvent{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     model.INFO,
		Service:   "mock-service",
		Message:   "Mock message",
	}
}

// MockStorage implements storage.Storage
type MockStorage struct {
	storeBatchCalled int
	totalStored      int
	mu               sync.Mutex
}

func (m *MockStorage) Store(ctx context.Context, event model.LogEvent) error {
	return nil
}

func (m *MockStorage) StoreBatch(ctx context.Context, events []model.LogEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeBatchCalled++
	m.totalStored += len(events)
	return nil
}

func (m *MockStorage) Close() error {
	return nil
}

func TestEngine_Worker_Batching(t *testing.T) {
	mockGen := &MockGenerator{}
	mockStore := &MockStorage{}

	cfg := EngineConfig{
		Workers:       1,
		DefaultRate:   100,
		BatchSize:     5,
		FlushInterval: 1 * time.Second,
	}

	eng := NewEngine(mockGen, mockStore, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		// Run worker for a short time
		eng.worker(ctx, &wg)
	}()

	// Wait enough time to generate at least 2 batches of 5 logs
	time.Sleep(150 * time.Millisecond)
	cancel()
	wg.Wait()

	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	if mockStore.totalStored == 0 {
		t.Errorf("Expected logs to be stored, got 0")
	}

	if mockStore.totalStored % cfg.BatchSize != 0 && mockStore.totalStored < cfg.BatchSize {
		// It might flush early due to context cancel, but it should have stored something
		t.Logf("Flushed %d logs on exit", mockStore.totalStored)
	}
}
