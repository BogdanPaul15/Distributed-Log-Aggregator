package storage

import (
	"context"

	"log-generator/internal/model"
)

type Storage interface {
	Store(ctx context.Context, event model.LogEvent) error
	StoreBatch(ctx context.Context, events []model.LogEvent) error
	Close() error
}
