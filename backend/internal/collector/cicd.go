package collector

import (
	"context"
	"valiant/internal/domain"
)

type CICDCollector struct {
	webhookSecret string
}

func NewCICDCollector(webhookSecret string) *CICDCollector {
	return &CICDCollector{
		webhookSecret: webhookSecret,
	}
}

func (c *CICDCollector) Start(ctx context.Context, eventChan chan<- domain.ChangeEvent) error {
	// CICD changes are usually pushed via webhook, so this might be a no-op
	// or it might poll a CI system's API.
	// For webhooks, the API handler (router.go) receives them directly.
	// This collector could be used for polling if needed.
	return nil
}

func (c *CICDCollector) Name() string {
	return "ci-cd"
}
