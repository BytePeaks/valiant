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
	// TODO: Implement git log / tag inspection
	// In a real implementation, we might use go-git or exec git command
	return []domain.ChangeEvent{}, nil
}

func (c *GitCollector) Name() string {
	return "git"
}
