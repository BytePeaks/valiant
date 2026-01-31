package collector

import (
	"context"
	"valiant/internal/domain"
)

// Collector defines the interface for gathering change events from various sources.
type Collector interface {
	Start(ctx context.Context, eventChan chan<- domain.ChangeEvent) error
	Name() string
}
