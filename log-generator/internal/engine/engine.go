package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"log-generator/internal/generator"
	"log-generator/internal/model"
	"log-generator/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
)

var (
	logsGenerated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "log_generator_logs_generated_total",
		Help: "The total number of logs generated",
	}, []string{"service", "level"})
	storageDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "log_generator_storage_duration_seconds",
		Help:    "Time taken to store log events",
		Buckets: prometheus.DefBuckets,
	})
	storageError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "log_generator_storage_errors_total",
		Help: "Total number of storage failures",
	})
	activeWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "log_generator_active_workers",
		Help: "Number of currently running worker goroutines",
	})
)

type EngineConfig struct {
	Workers       int           `yaml:"workers"`
	DefaultRate   int           `yaml:"default_rate"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

type Engine struct {
	generator generator.Generator
	storage   storage.Storage
	config    EngineConfig
	limiter   *rate.Limiter
}

func NewEngine(generator generator.Generator, storage storage.Storage, cfg EngineConfig) *Engine {
	limiter := rate.NewLimiter(rate.Limit(cfg.DefaultRate), cfg.Workers)

	return &Engine{
		generator: generator,
		storage:   storage,
		config:    cfg,
		limiter:   limiter,
	}
}

func (e *Engine) Start(ctx context.Context) {
	var wg sync.WaitGroup

	log.Printf("Engine starting with %d workers at %d logs/sec", e.config.Workers, e.config.DefaultRate)

	for i := 0; i < e.config.Workers; i++ {
		wg.Add(1)
		go e.worker(ctx, &wg)
	}

	wg.Wait()

	if err := e.storage.Close(); err != nil {
		log.Printf("Error closing storage: %v", err)
	}

	log.Println("Engine stopped.")
}

func (e *Engine) SetRate(newRate int) {
	e.limiter.SetLimit(rate.Limit(newRate))
	log.Printf("Engine target rate updated to %d logs/sec", newRate)
}

func (e *Engine) worker(ctx context.Context, wg *sync.WaitGroup) {
	activeWorkers.Inc()
	defer activeWorkers.Dec()
	defer wg.Done()

	batch := make([]model.LogEvent, 0, e.config.BatchSize)
	lastFlush := time.Now()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		start := time.Now()
		if err := e.storage.StoreBatch(ctx, batch); err != nil {
			storageError.Inc()
			log.Printf("Worker failed to store batch: %v", err)
		}
		storageDuration.Observe(time.Since(start).Seconds())

		// Reset batch
		batch = make([]model.LogEvent, 0, e.config.BatchSize)
		lastFlush = time.Now()
	}

	for {
		if err := e.limiter.Wait(ctx); err != nil {
			flush() // Try to flush remaining logs on exit
			return
		}

		event := e.generator.Generate()
		logsGenerated.WithLabelValues(event.Service, string(event.Level)).Inc()

		batch = append(batch, event)

		if len(batch) >= e.config.BatchSize || time.Since(lastFlush) >= e.config.FlushInterval {
			flush()
		}
	}
}
