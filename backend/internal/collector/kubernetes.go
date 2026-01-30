package collector

import (
	"context"
	"valiant/internal/domain"
)

type KubernetesCollector struct {
	kubeConfigPath string
}

func NewKubernetesCollector(kubeConfigPath string) *KubernetesCollector {
	return &KubernetesCollector{
		kubeConfigPath: kubeConfigPath,
	}
}

func (c *KubernetesCollector) Collect(ctx context.Context) ([]domain.ChangeEvent, error) {
	// TODO: Implement Kubernetes polling/watching
	return []domain.ChangeEvent{}, nil
}

func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}
