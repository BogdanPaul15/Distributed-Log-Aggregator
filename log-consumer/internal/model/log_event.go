package model

type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

type LogEvent struct {
	Timestamp string         `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	Service   string         `json:"service"`
	TraceID   string         `json:"trace_id"`
	Message   string         `json:"message"`
	Payload   map[string]any `json:"payload,omitempty"`
}
