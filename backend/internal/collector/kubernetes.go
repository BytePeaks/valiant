package collector

import (
	"context"
	"fmt"
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesCollector struct {
	clientset      kubernetes.Interface
	kubeConfigPath string
	namespaces     []string
	requireAnnot   bool
	allowedSources []string
	lastProcessed  map[string]int64 // map["ns/name"]generation
}

func NewKubernetesCollector(cfg config.Config, clientset kubernetes.Interface) (*KubernetesCollector, error) {
	if clientset == nil {
		var restCfg *rest.Config
		var err error

		if cfg.Kubernetes.KubeConfigPath != "" {
			restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubernetes.KubeConfigPath)
		} else {
			restCfg, err = rest.InClusterConfig()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}

		clientset, err = kubernetes.NewForConfig(restCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
		}
	}

	return &KubernetesCollector{
		clientset:      clientset,
		kubeConfigPath: cfg.Kubernetes.KubeConfigPath,
		namespaces:     cfg.Kubernetes.Namespaces,
		requireAnnot:   cfg.Kubernetes.RequireAnnotation,
		allowedSources: cfg.Kubernetes.AllowedSources,
		lastProcessed:  make(map[string]int64),
	}, nil
}

func (c *KubernetesCollector) Start(ctx context.Context, eventChan chan<- domain.ChangeEvent) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial run
	c.collectAndSend(ctx, eventChan)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.collectAndSend(ctx, eventChan)
		}
	}
}

func (c *KubernetesCollector) collectAndSend(ctx context.Context, eventChan chan<- domain.ChangeEvent) {
	allDeps, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing deployments: %v\n", err)
		return
	}

	watchedNamespaces := make(map[string]bool)
	for _, ns := range c.namespaces {
		watchedNamespaces[ns] = true
	}

	for _, d := range allDeps.Items {
		key := fmt.Sprintf("%s/%s", d.Namespace, d.Name)

		// Namespace Filter
		if len(c.namespaces) > 0 && !watchedNamespaces[d.Namespace] {
			if isRecentRollout(&d) && c.lastProcessed[key] < d.Generation {
				fmt.Printf("Ignored rollout: deployment %s in namespace %s is not in watched list\n", d.Name, d.Namespace)
				c.lastProcessed[key] = d.Generation
			}
			continue
		}

		// Deduplicate
		if gen, ok := c.lastProcessed[key]; ok && gen >= d.Generation {
			continue
		}

		// 1. Intent Validation
		source, exists := d.Annotations["valiant.io/source"]
		if c.requireAnnot {
			if !exists || !c.isSourceAllowed(source) {
				continue
			}
		}

		// 2. Completion Validation
		var progressingCond, availableCond *appsv1.DeploymentCondition
		for i := range d.Status.Conditions {
			cond := &d.Status.Conditions[i]
			if cond.Type == "Progressing" {
				progressingCond = cond
			}
			if cond.Type == "Available" {
				availableCond = cond
			}
		}

		if availableCond == nil || availableCond.Status != "True" || 
		   progressingCond == nil || progressingCond.Reason != "NewReplicaSetAvailable" {
			continue
		}

		// 3. Timing & Metadata
		if time.Since(availableCond.LastUpdateTime.Time) < 35*time.Second {
			rolloutStartTime := d.CreationTimestamp.Time
			
			var rsSelector string
			if d.Spec.Selector != nil {
				rsSelector = metav1.FormatLabelSelector(d.Spec.Selector)
			}

			rss, err := c.clientset.AppsV1().ReplicaSets(d.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: rsSelector,
			})
			if err == nil && len(rss.Items) > 0 {
				var newestRS *appsv1.ReplicaSet
				for i := range rss.Items {
					rs := &rss.Items[i]
					if newestRS == nil || rs.CreationTimestamp.After(newestRS.CreationTimestamp.Time) {
						newestRS = rs
					}
				}
				if newestRS != nil {
					rolloutStartTime = newestRS.CreationTimestamp.Time
				}
			}

			rolloutEndTime := availableCond.LastUpdateTime.Time
			gitSha := d.Annotations["valiant.io/git-sha"]
			if gitSha == "" {
				gitSha = d.Annotations["kubernetes.io/change-cause"]
			}

			image := ""
			if len(d.Spec.Template.Spec.Containers) > 0 {
				image = d.Spec.Template.Spec.Containers[0].Image
			}

			eventChan <- domain.ChangeEvent{
				ID:          fmt.Sprintf("k8s-%s-%s-%d", d.Namespace, d.Name, d.Generation),
				TriggerType: "GitOps",
				ExecutionID: fmt.Sprintf("%s-%d", d.UID, d.Generation),
				ChangeType:  "deployment_rollout",
				Timestamp:   rolloutStartTime,
				EndTime:     &rolloutEndTime,
				AffectedServices: []string{d.Name},
				Summary:     fmt.Sprintf("Deployment %s rollout completed via %s", d.Name, source),
				Metadata: map[string]string{
					"namespace":     d.Namespace,
					"kind":          "Deployment",
					"generation":    fmt.Sprintf("%d", d.Generation),
					"image":         image,
					"intent_source": source,
					"git_sha":       gitSha,
					"rollout_start": rolloutStartTime.Format(time.RFC3339),
					"rollout_end":   rolloutEndTime.Format(time.RFC3339),
				},
			}
			c.lastProcessed[key] = d.Generation
		}
	}
}
func isRecentRollout(d *appsv1.Deployment) bool {
	for _, cond := range d.Status.Conditions {
		if cond.Type == "Available" && cond.Status == "True" {
			return time.Since(cond.LastUpdateTime.Time) < 35*time.Second
		}
	}
	return false
}

func (c *KubernetesCollector) isSourceAllowed(source string) bool {
	for _, s := range c.allowedSources {
		if s == source {
			return true
		}
	}
	return false
}

func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}
