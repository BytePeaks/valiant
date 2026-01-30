package collector

import (
	"context"
	"fmt"
	"time"
	"valiant/internal/domain"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesCollector struct {
	clientset      *kubernetes.Clientset
	kubeConfigPath string
}

func NewKubernetesCollector(kubeConfigPath string) (*KubernetesCollector, error) {
	var config *rest.Config
	var err error

	if kubeConfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &KubernetesCollector{
		clientset:      clientset,
		kubeConfigPath: kubeConfigPath,
	}, nil
}

func (c *KubernetesCollector) Collect(ctx context.Context) ([]domain.ChangeEvent, error) {
	// For MVP, we'll list deployments and look for recent changes.
	// In a real implementation, we'd use an Informer/Watch.
	
	deps, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var events []domain.ChangeEvent
	for _, d := range deps.Items {
		// This is a simplified heuristic: if the deployment was updated in the last 5 minutes.
		// A better way is to track ObservedGeneration or use Events API.
		for _, cond := range d.Status.Conditions {
			if cond.Type == "Progressing" && cond.Reason == "NewReplicaSetAvailable" {
				if time.Since(cond.LastUpdateTime.Time) < 5*time.Minute {
					events = append(events, domain.ChangeEvent{
						ID:               string(d.UID),
						Source:           "kubernetes",
						ChangeType:       "deployment_rollout",
						Timestamp:        cond.LastUpdateTime.Time,
						AffectedServices: []string{d.Name},
						Summary:          fmt.Sprintf("Deployment %s rolled out", d.Name),
						Metadata: map[string]string{
							"namespace": d.Namespace,
							"image":     d.Spec.Template.Spec.Containers[0].Image,
						},
					})
				}
			}
		}
	}

	return events, nil
}

func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}
