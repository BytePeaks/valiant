package collector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesCollector struct {
	clientset         kubernetes.Interface
	kubeConfigPath    string
	namespaces        []string
	requireAnnot      bool
	allowedSources    []string
	watchConfigMaps   bool
	watchSecrets      bool
	lastProcessed     map[string]int64  // map["ns/name"]generation
	lastProcessedSha  map[string]string // map["ns/name"]fingerprint
	lastConfigMapHash map[string]string // map["ns/name"]sha256 of .data
	lastSecretHash    map[string]string // map["ns/name"]sha256 of .data
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
		clientset:         clientset,
		kubeConfigPath:    cfg.Kubernetes.KubeConfigPath,
		namespaces:        cfg.Kubernetes.Namespaces,
		requireAnnot:      cfg.Kubernetes.RequireAnnotation,
		allowedSources:    cfg.Kubernetes.AllowedSources,
		watchConfigMaps:   cfg.Kubernetes.WatchConfigMaps,
		watchSecrets:      cfg.Kubernetes.WatchSecrets,
		lastProcessed:     make(map[string]int64),
		lastProcessedSha:  make(map[string]string),
		lastConfigMapHash: make(map[string]string),
		lastSecretHash:    make(map[string]string),
	}, nil
}

func (c *KubernetesCollector) Start(ctx context.Context, eventChan chan<- domain.ChangeEvent) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial run
	c.CollectAndSend(ctx, eventChan)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.CollectAndSend(ctx, eventChan)
		}
	}
}

// listDeployments lists deployments across the configured namespaces, handling permission errors gracefully.
func (c *KubernetesCollector) listDeployments(ctx context.Context) ([]appsv1.Deployment, map[string]bool) {
	watchedNamespaces := make(map[string]bool)
	for _, ns := range c.namespaces {
		watchedNamespaces[ns] = true
	}

	if len(c.namespaces) == 0 {
		// Cluster-wide listing
		allDeps, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list Deployments cluster-wide. "+
					"Apply deploy/kubernetes/rbac.yaml or configure specific namespaces in config.yaml. Error: %v\n", err)
			} else {
				fmt.Printf("Error listing deployments: %v\n", err)
			}
			return nil, watchedNamespaces
		}
		return allDeps.Items, watchedNamespaces
	}

	// Per-namespace listing for graceful permission handling
	var allItems []appsv1.Deployment
	for _, ns := range c.namespaces {
		deps, err := c.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list Deployments in namespace %q. "+
					"Apply deploy/kubernetes/rbac.yaml to enable visibility for this namespace.\n", ns)
			} else {
				fmt.Printf("Error listing deployments in namespace %s: %v\n", ns, err)
			}
			continue
		}
		allItems = append(allItems, deps.Items...)
	}
	return allItems, watchedNamespaces
}

func (c *KubernetesCollector) CollectAndSend(ctx context.Context, eventChan chan<- domain.ChangeEvent) {
	allDepItems, watchedNamespaces := c.listDeployments(ctx)
	if allDepItems == nil {
		allDepItems = []appsv1.Deployment{}
	}

	for _, d := range allDepItems {
		key := fmt.Sprintf("%s/%s", d.Namespace, d.Name)

		// Namespace Filter
		if len(c.namespaces) > 0 && !watchedNamespaces[d.Namespace] {
			if isRecentRollout(&d) && c.lastProcessed[key] < d.Generation {
				fmt.Printf("Ignored rollout: deployment %s in namespace %s is not in watched list\n", d.Name, d.Namespace)
				c.lastProcessed[key] = d.Generation
			}
			continue
		}

		// Deduplicate based on generation
		if gen, ok := c.lastProcessed[key]; ok && gen >= d.Generation {
			continue
		}

		// Deduplicate based on fingerprint
		gitSha := d.Annotations["valiant.io/git-sha"]
		if gitSha == "" {
			gitSha = d.Annotations["kubernetes.io/change-cause"]
		}
		if gitSha != "" {
			if lastSha, ok := c.lastProcessedSha[key]; ok && lastSha == gitSha {
				fmt.Printf("Ignored rollout: deployment %s generation changed but fingerprint %s is the same\n", key, gitSha)
				c.lastProcessed[key] = d.Generation // Update generation to prevent re-evaluating
				continue
			}
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
			image := ""
			if len(d.Spec.Template.Spec.Containers) > 0 {
				image = d.Spec.Template.Spec.Containers[0].Image
			}

			eventChan <- domain.ChangeEvent{
				ID:               fmt.Sprintf("k8s-%s-%s-%d", d.Namespace, d.Name, d.Generation),
				TriggerType:      "GitOps",
				ExecutionID:      fmt.Sprintf("%s-%d", d.UID, d.Generation),
				ChangeType:       "deployment_rollout",
				Timestamp:        rolloutStartTime,
				EndTime:          &rolloutEndTime,
				AffectedServices: []string{d.Name},
				Summary:          fmt.Sprintf("Deployment %s rollout completed via %s", d.Name, source),
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
			if gitSha != "" {
				c.lastProcessedSha[key] = gitSha
			}
		}
	}

	if c.watchConfigMaps {
		c.collectConfigMapChanges(ctx, eventChan, watchedNamespaces, allDepItems)
	}
	if c.watchSecrets {
		c.collectSecretChanges(ctx, eventChan, watchedNamespaces, allDepItems)
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

func hashConfigMapData(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(data[k]))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashSecretData(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write(data[k])
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func findReferencingDeployments(deps []appsv1.Deployment, name, kind string) []string {
	var result []string
	for _, d := range deps {
		if referencesResource(d, name, kind) {
			result = append(result, d.Name)
		}
	}
	return result
}

func referencesResource(d appsv1.Deployment, name, kind string) bool {
	podSpec := d.Spec.Template.Spec

	// Check envFrom
	for _, c := range podSpec.Containers {
		for _, ef := range c.EnvFrom {
			if kind == "ConfigMap" && ef.ConfigMapRef != nil && ef.ConfigMapRef.Name == name {
				return true
			}
			if kind == "Secret" && ef.SecretRef != nil && ef.SecretRef.Name == name {
				return true
			}
		}
		// Check env[].valueFrom
		for _, e := range c.Env {
			if e.ValueFrom == nil {
				continue
			}
			if kind == "ConfigMap" && e.ValueFrom.ConfigMapKeyRef != nil && e.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
			if kind == "Secret" && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}

	// Check volumes
	for _, v := range podSpec.Volumes {
		if kind == "ConfigMap" && v.ConfigMap != nil && v.ConfigMap.Name == name {
			return true
		}
		if kind == "Secret" && v.Secret != nil && v.Secret.SecretName == name {
			return true
		}
	}

	return false
}

func (c *KubernetesCollector) listConfigMaps(ctx context.Context, watchedNamespaces map[string]bool) []corev1.ConfigMap {
	if len(c.namespaces) == 0 {
		cms, err := c.clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list ConfigMaps cluster-wide. "+
					"Apply deploy/kubernetes/rbac.yaml or configure specific namespaces.\n")
			} else {
				fmt.Printf("Error listing configmaps: %v\n", err)
			}
			return nil
		}
		return cms.Items
	}

	var items []corev1.ConfigMap
	for _, ns := range c.namespaces {
		cms, err := c.clientset.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list ConfigMaps in namespace %q. "+
					"Apply deploy/kubernetes/rbac.yaml to enable visibility for this namespace.\n", ns)
			} else {
				fmt.Printf("Error listing configmaps in namespace %s: %v\n", ns, err)
			}
			continue
		}
		items = append(items, cms.Items...)
	}
	return items
}

func (c *KubernetesCollector) collectConfigMapChanges(ctx context.Context, eventChan chan<- domain.ChangeEvent, watchedNamespaces map[string]bool, deps []appsv1.Deployment) {
	cmItems := c.listConfigMaps(ctx, watchedNamespaces)

	for _, cm := range cmItems {
		// Namespace filter
		if len(c.namespaces) > 0 && !watchedNamespaces[cm.Namespace] {
			continue
		}

		// Annotation filter
		if c.requireAnnot {
			source, exists := cm.Annotations["valiant.io/source"]
			if !exists || !c.isSourceAllowed(source) {
				continue
			}
		}

		key := fmt.Sprintf("%s/%s", cm.Namespace, cm.Name)
		hash := hashConfigMapData(cm.Data)

		prevHash, seen := c.lastConfigMapHash[key]
		c.lastConfigMapHash[key] = hash

		// First-seen: store hash, skip event
		if !seen {
			continue
		}

		// No change
		if prevHash == hash {
			continue
		}

		// Data changed — emit event
		affectedServices := findReferencingDeployments(deps, cm.Name, "ConfigMap")

		dataKeys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)

		eventChan <- domain.ChangeEvent{
			ID:               fmt.Sprintf("k8s-cm-%s-%s-%s", cm.Namespace, cm.Name, hash[:8]),
			TriggerType:      "GitOps",
			ChangeType:       "configmap_update",
			Timestamp:        time.Now(),
			AffectedServices: affectedServices,
			Summary:          fmt.Sprintf("ConfigMap %s/%s data changed", cm.Namespace, cm.Name),
			Metadata: map[string]string{
				"namespace":                cm.Namespace,
				"kind":                     "ConfigMap",
				"data_keys":               strings.Join(dataKeys, ","),
				"data_hash":               hash[:8],
				"intent_source":           cm.Annotations["valiant.io/source"],
				"referencing_deployments": strings.Join(affectedServices, ","),
			},
		}
	}
}

func (c *KubernetesCollector) listSecrets(ctx context.Context, watchedNamespaces map[string]bool) []corev1.Secret {
	if len(c.namespaces) == 0 {
		secrets, err := c.clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list Secrets cluster-wide. "+
					"Apply deploy/kubernetes/rbac.yaml or configure specific namespaces.\n")
			} else {
				fmt.Printf("Error listing secrets: %v\n", err)
			}
			return nil
		}
		return secrets.Items
	}

	var items []corev1.Secret
	for _, ns := range c.namespaces {
		secrets, err := c.clientset.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list Secrets in namespace %q. "+
					"Apply deploy/kubernetes/rbac.yaml to enable visibility for this namespace.\n", ns)
			} else {
				fmt.Printf("Error listing secrets in namespace %s: %v\n", ns, err)
			}
			continue
		}
		items = append(items, secrets.Items...)
	}
	return items
}

func (c *KubernetesCollector) collectSecretChanges(ctx context.Context, eventChan chan<- domain.ChangeEvent, watchedNamespaces map[string]bool, deps []appsv1.Deployment) {
	secretItems := c.listSecrets(ctx, watchedNamespaces)

	for _, s := range secretItems {
		// Skip ServiceAccountToken type secrets
		if s.Type == corev1.SecretTypeServiceAccountToken {
			continue
		}

		// Namespace filter
		if len(c.namespaces) > 0 && !watchedNamespaces[s.Namespace] {
			continue
		}

		// Annotation filter
		if c.requireAnnot {
			source, exists := s.Annotations["valiant.io/source"]
			if !exists || !c.isSourceAllowed(source) {
				continue
			}
		}

		key := fmt.Sprintf("%s/%s", s.Namespace, s.Name)
		hash := hashSecretData(s.Data)

		prevHash, seen := c.lastSecretHash[key]
		c.lastSecretHash[key] = hash

		// First-seen: store hash, skip event
		if !seen {
			continue
		}

		// No change
		if prevHash == hash {
			continue
		}

		// Data changed — emit event
		affectedServices := findReferencingDeployments(deps, s.Name, "Secret")

		dataKeys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)

		eventChan <- domain.ChangeEvent{
			ID:               fmt.Sprintf("k8s-secret-%s-%s-%s", s.Namespace, s.Name, hash[:8]),
			TriggerType:      "GitOps",
			ChangeType:       "secret_update",
			Timestamp:        time.Now(),
			AffectedServices: affectedServices,
			Summary:          fmt.Sprintf("Secret %s/%s data changed", s.Namespace, s.Name),
			Metadata: map[string]string{
				"namespace":                s.Namespace,
				"kind":                     "Secret",
				"data_keys":               strings.Join(dataKeys, ","),
				"intent_source":           s.Annotations["valiant.io/source"],
				"referencing_deployments": strings.Join(affectedServices, ","),
			},
		}
	}
}

func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}
