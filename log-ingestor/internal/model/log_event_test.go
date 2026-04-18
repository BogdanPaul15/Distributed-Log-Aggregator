package model

import (
	"encoding/json"
	"testing"
)

func TestLogEvent_MarshalJSON(t *testing.T) {
	event := LogEvent{
		Timestamp: "2023-01-01T12:00:00Z",
		Level:     INFO,
		Service:   "test-service",
		TraceID:   "12345",
		Message:   "Test message",
		Payload: map[string]any{
			"key": "value",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal LogEvent: %v", err)
	}

	var unmarshaled LogEvent
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal LogEvent: %v", err)
	}

	if unmarshaled.Level != event.Level {
		t.Errorf("Expected level %v, got %v", event.Level, unmarshaled.Level)
	}
	if unmarshaled.Service != event.Service {
		t.Errorf("Expected service %v, got %v", event.Service, unmarshaled.Service)
	}
	if unmarshaled.Message != event.Message {
		t.Errorf("Expected message %v, got %v", event.Message, unmarshaled.Message)
	}
}
