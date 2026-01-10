package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"log-ingestor/internal/kafka"
	"log-ingestor/internal/metrics"
	"log-ingestor/internal/model"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	producer *kafka.Producer
}

func NewServer(producer *kafka.Producer) *Server {
	return &Server{
		producer: producer,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", s.handleLogs)
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		metrics.HttpRequestsTotal.WithLabelValues(strconv.Itoa(http.StatusMethodNotAllowed), r.Method).Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var events []model.LogEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		metrics.HttpRequestsTotal.WithLabelValues(strconv.Itoa(http.StatusBadRequest), r.Method).Inc()
		// Fallback: try to decode a single event for backward compatibility
		// Note: This requires resetting the body reader or using a buffer if we already read from it.
		// Since we can't easily rewind r.Body, and the generator is updated to send batches,
		// we will assume batch format (JSON array) is required.
		http.Error(w, "Invalid request body (expected JSON array)", http.StatusBadRequest)
		return
	}

	if err := s.producer.ProduceBatch(r.Context(), events); err != nil {
		log.Printf("Failed to produce batch logs: %v", err)
		metrics.HttpRequestsTotal.WithLabelValues(strconv.Itoa(http.StatusInternalServerError), r.Method).Inc()
		http.Error(w, "Failed to process log batch", http.StatusInternalServerError)
		return
	}

	for _, event := range events {
		metrics.LogsProcessed.WithLabelValues(string(event.Level), event.Service, "success").Inc()
	}

	metrics.HttpRequestsTotal.WithLabelValues(strconv.Itoa(http.StatusAccepted), r.Method).Inc()
	w.WriteHeader(http.StatusAccepted)
}
