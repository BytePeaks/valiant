package storage

import (
	"context"
	"valiant/internal/domain"
)

// Storage defines the interface for persisting and retrieving change events.
type Storage interface {
	SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error
	GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, error)
	GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error)
}
