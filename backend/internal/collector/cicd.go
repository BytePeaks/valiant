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

func (c *CICDCollector) Collect(ctx context.Context) ([]domain.ChangeEvent, error) {
	// CICD changes are usually pushed via webhook, so this might be a no-op 
	// or it might poll a CI system's API.
	return []domain.ChangeEvent{}, nil
}

func (c *CICDCollector) Name() string {
	return "ci-cd"
}
