package collector

import (
	"context"
	"valiant/internal/domain"
)

type GitCollector struct {
	repoPath string
}

func NewGitCollector(repoPath string) *GitCollector {
	return &GitCollector{
		repoPath: repoPath,
	}
}

func (c *GitCollector) Collect(ctx context.Context) ([]domain.ChangeEvent, error) {
	// GitCollector is disabled in favor of execution-based monitoring (CI/CD, K8s).
	// Git tags/commits should be metadata on those execution events, not standalone events.
	return []domain.ChangeEvent{}, nil
}

func (c *GitCollector) Name() string {
	return "git"
}
