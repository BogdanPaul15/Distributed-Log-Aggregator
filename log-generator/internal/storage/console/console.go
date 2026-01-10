package console

import (
	"context"
	"encoding/json"
	"fmt"

	"log-generator/internal/model"
)

type ConsoleStorage struct{}

func NewConsoleStorage() *ConsoleStorage {
	return &ConsoleStorage{}
}

func (cs *ConsoleStorage) Store(ctx context.Context, event model.LogEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	_, err = fmt.Println(string(data))
	return err
}

func (cs *ConsoleStorage) Close() error {
	return nil
}

func (cs *ConsoleStorage) StoreBatch(ctx context.Context, events []model.LogEvent) error {
	for _, event := range events {
		if err := cs.Store(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
