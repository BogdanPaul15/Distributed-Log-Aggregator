package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"log-generator/internal/engine"
	"log-generator/internal/generator/random"
	"log-generator/internal/model"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	eng *engine.Engine
	gen *random.RandomGenerator
}

func NewServer(eng *engine.Engine, gen *random.RandomGenerator) *Server {
	return &Server{
		eng: eng,
		gen: gen,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rate", s.handleRate)
	mux.HandleFunc("/weights", s.handleWeights)
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Rate int `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	s.eng.SetRate(req.Rate)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Rate updated to %d logs/sec\n", req.Rate)
}

func (s *Server) handleWeights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var weights map[model.LogLevel]int
	if err := json.NewDecoder(r.Body).Decode(&weights); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	s.gen.SetWeights(weights)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Weights updated\n"))
}
