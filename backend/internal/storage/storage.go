package storage

import (
	"context"
	"time"
	"valiant/internal/domain"
)

// Storage defines the interface for persisting and retrieving change events.
type Storage interface {
	SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error
	GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error)
	GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error)
	GetServices(ctx context.Context, namespace string, linkedOnly bool) ([]string, error)
	GetNamespaces(ctx context.Context) ([]string, error)
	GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error)

	SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error
	GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error)
	GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error)

	GetServicePreferences(ctx context.Context, serviceName string) ([]string, error)
	SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error

	DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error)

	// Event linking methods
	SaveEventLink(ctx context.Context, link domain.EventLink) error
	GetEventLinksByEventID(ctx context.Context, eventID string) ([]domain.EventLink, error)
	GetEventLinksByExecutionID(ctx context.Context, executionEventID string) ([]domain.EventLink, error)
	GetRecentConfigChangeEvents(ctx context.Context, service string, before time.Time, within time.Duration) ([]domain.ChangeEvent, error)
}
