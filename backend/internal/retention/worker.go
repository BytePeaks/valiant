package retention

import (
	"context"
	"log"
	"time"
	"valiant/internal/storage"
)

type Worker struct {
	storage storage.Storage
	ttl     time.Duration
}

func NewWorker(s storage.Storage, ttl time.Duration) *Worker {
	return &Worker{
		storage: s,
		ttl:     ttl,
	}
}

func (w *Worker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting retention worker (ttl=%s, interval=%s)...", w.ttl, interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-w.ttl)
			deleted, err := w.storage.DeleteChangeEventsOlderThan(ctx, cutoff)
			if err != nil {
				log.Printf("Retention worker error: %v", err)
				continue
			}
			if deleted > 0 {
				log.Printf("Retention worker: deleted %d events older than %s", deleted, cutoff.Format(time.RFC3339))
			}
		}
	}
}
