package collector

import (
	"context"
	"valiant/internal/domain"
)

// Collector defines the interface for gathering change events from various sources.
type Collector interface {
	Collect(ctx context.Context) ([]domain.ChangeEvent, error)
	Name() string
}
