package correlator

import (
	"context"
	"log"
	"time"
)

type Worker struct {
	engine          *Engine
	pollingInterval time.Duration
}

func NewWorker(e *Engine, pollingInterval time.Duration) *Worker {
	return &Worker{
		engine:          e,
		pollingInterval: pollingInterval,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollingInterval)
	defer ticker.Stop()

	log.Println("Starting background analysis worker...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunBatchPublic(ctx)
		}
	}
}

func (w *Worker) RunBatchPublic(ctx context.Context) {
	events, err := w.engine.storage.GetEventsPendingAnalysis(ctx)
	if err != nil {
		log.Printf("Worker error: failed to fetch pending events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	log.Printf("Worker: processing %d pending events for analysis...", len(events))

	for _, event := range events {
		_, err := w.engine.AnalyzeImpact(ctx, event)
		if err != nil {
			if err == ErrImpactWindowNotClosed {
				// Too early, ignore for now
				continue
			}
			log.Printf("Worker: failed to analyze event %s: %v", event.ID, err)
		} else {
			log.Printf("Worker: automatically analyzed and snapshotted event %s", event.ID)
		}
	}
}
