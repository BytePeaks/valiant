package retention

import (
	"context"
	"log"
	"time"
	"valiant/internal/storage"
)

type Worker struct {
	storage storage.Storage
	getTTL  func() time.Duration
}

func NewWorker(s storage.Storage, getTTL func() time.Duration) *Worker {
	return &Worker{
		storage: s,
		getTTL:  getTTL,
	}
}

func (w *Worker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ttl := w.getTTL()
	log.Printf("Starting retention worker (ttl=%s, interval=%s)...", ttl, interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-w.getTTL())
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
