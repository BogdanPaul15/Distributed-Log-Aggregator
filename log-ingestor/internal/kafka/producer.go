package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"log-ingestor/internal/model"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
		Async:    true,
	}

	return &Producer{writer: w}
}

func (p *Producer) Produce(ctx context.Context, event model.LogEvent) error {
	key := event.TraceID
	if key == "" {
		key = event.Service
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte(key),
			Value: value,
			Time:  time.Now(),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (p *Producer) ProduceBatch(ctx context.Context, events []model.LogEvent) error {
	msgs := make([]kafka.Message, len(events))
	for i, event := range events {
		key := event.TraceID
		if key == "" {
			key = event.Service
		}

		value, err := json.Marshal(event)
		if err != nil {
			return err
		}

		msgs[i] = kafka.Message{
			Key:   []byte(key),
			Value: value,
			Time:  time.Now(),
		}
	}

	return p.writer.WriteMessages(ctx, msgs...)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
