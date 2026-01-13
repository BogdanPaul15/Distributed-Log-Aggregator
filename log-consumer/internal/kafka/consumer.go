package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"log-consumer/internal/model"
	"log-consumer/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
)

var (
	messagesConsumed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "The total number of messages consumed from Kafka",
	})
	consumerLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kafka_consumer_lag",
		Help: "The current lag of the consumer group",
	})
)

type Consumer struct {
	reader  *kafka.Reader
	storage *storage.OpenSearchClient
}

func NewConsumer(brokers []string, topic string, groupID string, storage *storage.OpenSearchClient) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	return &Consumer{
		reader:  r,
		storage: storage,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	defer c.reader.Close()
	log.Println("Starting Kafka consumer with batch processing...")

	// Start background metric updater
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := c.reader.Stats()
				consumerLag.Set(float64(stats.Lag))
			}
		}
	}()

	const (
		batchSize     = 500
		flushInterval = 1 * time.Second
	)

	batchEvents := make([]model.LogEvent, 0, batchSize)
	batchMessages := make([]kafka.Message, 0, batchSize)
	lastFlush := time.Now()

	flush := func() error {
		if len(batchEvents) == 0 {
			return nil
		}

		if err := c.storage.IndexBatch(ctx, batchEvents); err != nil {
			log.Printf("Failed to index batch of %d logs: %v", len(batchEvents), err)
			return err
		}

		// Commit offsets after successful indexing
		if err := c.reader.CommitMessages(ctx, batchMessages...); err != nil {
			log.Printf("Failed to commit messages: %v", err)
			// Non-fatal, but could lead to duplicates
		}

		messagesConsumed.Add(float64(len(batchEvents)))

		// Reset batches
		batchEvents = batchEvents[:0]
		batchMessages = batchMessages[:0]
		lastFlush = time.Now()
		return nil
	}

	for {
		// Calculate deadline for next read
		deadline := time.Now().Add(flushInterval)
		if len(batchEvents) > 0 {
			// If we have data, we want to respect the flush interval from the first message
			timeSinceFlush := time.Since(lastFlush)
			if timeSinceFlush >= flushInterval {
				if err := flush(); err != nil {
					// Decide how to handle flush errors (retry, log, etc)
				}
				continue
			}
			deadline = lastFlush.Add(flushInterval)
		}

		// Create a context with deadline for this fetch
		fetchCtx, cancel := context.WithDeadline(ctx, deadline)
		m, err := c.reader.FetchMessage(fetchCtx)
		cancel() // Good practice to cancel derived contexts

		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err() // specific check for parent context cancellation
			}

			// Check for timeout
			if isTimeout(err) {
				if err := flush(); err != nil {
					log.Printf("Flush failed on timeout: %v", err)
				}
				continue
			}

			log.Printf("Failed to fetch message: %v", err)
			time.Sleep(time.Second) // Backoff on error
			continue
		}

		// Reset deadline (though we set it every loop)
		// c.reader.SetReadDeadline(time.Time{})

		var event model.LogEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("Failed to unmarshal log event: %v", err)
			// We should probably still commit this message so we don't get stuck on it,
			// or send to DLQ. For now, let's just commit it individually or add to batch ignoring payload.
			// Ideally: c.reader.CommitMessages(ctx, m)
			continue
		}

		batchEvents = append(batchEvents, event)
		batchMessages = append(batchMessages, m)


		if len(batchEvents) >= batchSize {
			if err := flush(); err != nil {
				log.Printf("Flush failed on size limit: %v", err)
			}
		}
	}
}

func isTimeout(err error) bool {
	return err != nil && (err == context.DeadlineExceeded || err.Error() == "context deadline exceeded") // simplified check
}
