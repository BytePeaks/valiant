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

// WorkloadReferences holds references to workloads that reference a config resource.
type WorkloadReferences struct {
	Deployments  []string
	StatefulSets []string
}

type KubernetesCollector struct {
	clientset            kubernetes.Interface
	kubeConfigPath       string
	namespaces           []string
	requireAnnot         bool
	allowedSources       []string
	watchConfigMaps      bool
	watchSecrets         bool
	lastProcessed        map[string]int64  // map["ns/name"]generation (Deployments)
	lastProcessedSha     map[string]string // map["ns/name"]fingerprint (Deployments)
	lastProcessedImage   map[string]string // map["ns/name"]image tag (Deployments)
	lastProcessedSS      map[string]int64  // map["ns/name"]generation (StatefulSets)
	lastProcessedSSSha   map[string]string // map["ns/name"]fingerprint (StatefulSets)
	lastProcessedSSImage map[string]string // map["ns/name"]image tag (StatefulSets)
	lastConfigMapHash    map[string]string // map["ns/name"]sha256 of .data
	lastSecretHash       map[string]string // map["ns/name"]sha256 of .data
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
		clientset:            clientset,
		kubeConfigPath:       cfg.Kubernetes.KubeConfigPath,
		namespaces:           cfg.Kubernetes.Namespaces,
		requireAnnot:         cfg.Kubernetes.RequireAnnotation,
		allowedSources:       cfg.Kubernetes.AllowedSources,
		watchConfigMaps:      cfg.Kubernetes.WatchConfigMaps,
		watchSecrets:         cfg.Kubernetes.WatchSecrets,
		lastProcessed:        make(map[string]int64),
		lastProcessedSha:     make(map[string]string),
		lastProcessedImage:   make(map[string]string),
		lastProcessedSS:      make(map[string]int64),
		lastProcessedSSSha:   make(map[string]string),
		lastProcessedSSImage: make(map[string]string),
		lastConfigMapHash:    make(map[string]string),
		lastSecretHash:       make(map[string]string),
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

// listStatefulSets lists statefulsets across the configured namespaces, handling permission errors gracefully.
func (c *KubernetesCollector) listStatefulSets(ctx context.Context) ([]appsv1.StatefulSet, map[string]bool) {
	watchedNamespaces := make(map[string]bool)
	for _, ns := range c.namespaces {
		watchedNamespaces[ns] = true
	}

	if len(c.namespaces) == 0 {
		allSS, err := c.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list StatefulSets cluster-wide. "+
					"Apply deploy/kubernetes/rbac.yaml or configure specific namespaces in config.yaml. Error: %v\n", err)
			} else {
				fmt.Printf("Error listing statefulsets: %v\n", err)
			}
			return nil, watchedNamespaces
		}
		return allSS.Items, watchedNamespaces
	}

	var allItems []appsv1.StatefulSet
	for _, ns := range c.namespaces {
		ssets, err := c.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list StatefulSets in namespace %q. "+
					"Apply deploy/kubernetes/rbac.yaml to enable visibility for this namespace.\n", ns)
			} else {
				fmt.Printf("Error listing statefulsets in namespace %s: %v\n", ns, err)
			}
			continue
		}
		allItems = append(allItems, ssets.Items...)
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
		source := d.Annotations["valiant.io/source"]
		if c.requireAnnot {
			if !c.isSourceAllowed(source) {
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
			image, containerName := SelectAppContainer(d.Spec.Template.Spec.Containers, d.Annotations)
			previousImage := c.lastProcessedImage[key]

			metadata := map[string]string{
				"env":            d.Namespace,
				"namespace":      d.Namespace,
				"kind":           "Deployment",
				"generation":     fmt.Sprintf("%d", d.Generation),
				"image":          image,
				"image_tag":      image,
				"container_name": containerName,
				"intent_source":  source,
				"git_commit_sha": gitSha,
				"rollout_start":  rolloutStartTime.Format(time.RFC3339),
				"rollout_end":    rolloutEndTime.Format(time.RFC3339),
			}
			if previousImage != "" && previousImage != image {
				metadata["previous_image_tag"] = previousImage
			}

			eventChan <- domain.ChangeEvent{
				ID:               fmt.Sprintf("k8s-%s-%s-%d", d.Namespace, d.Name, d.Generation),
				Source:           "kubernetes",
				TriggerType:      c.getTriggerType(source),
				IsIntent:         false,
				IsExecution:      true,
				ExecutionID:      fmt.Sprintf("%s-%d", d.UID, d.Generation),
				ChangeType:       "deployment_rollout",
				Timestamp:        rolloutStartTime,
				EndTime:          &rolloutEndTime,
				AffectedServices: []string{d.Name},
				Summary:          fmt.Sprintf("Deployment %s rollout completed via %s", d.Name, source),
				Metadata:         metadata,
			}
			c.lastProcessed[key] = d.Generation
			c.lastProcessedImage[key] = image
			if gitSha != "" {
				c.lastProcessedSha[key] = gitSha
			}
		}
	}

	// StatefulSet processing
	allSSItems, _ := c.listStatefulSets(ctx)
	if allSSItems == nil {
		allSSItems = []appsv1.StatefulSet{}
	}

	for _, ss := range allSSItems {
		key := fmt.Sprintf("%s/%s", ss.Namespace, ss.Name)

		// Namespace Filter
		if len(c.namespaces) > 0 && !watchedNamespaces[ss.Namespace] {
			if isRecentStatefulSetRollout(&ss) && c.lastProcessedSS[key] < ss.Generation {
				fmt.Printf("Ignored rollout: statefulset %s in namespace %s is not in watched list\n", ss.Name, ss.Namespace)
				c.lastProcessedSS[key] = ss.Generation
			}
			continue
		}

		// Deduplicate based on generation
		if gen, ok := c.lastProcessedSS[key]; ok && gen >= ss.Generation {
			continue
		}

		// Deduplicate based on fingerprint
		gitSha := ss.Annotations["valiant.io/git-sha"]
		if gitSha == "" {
			gitSha = ss.Annotations["kubernetes.io/change-cause"]
		}
		if gitSha != "" {
			if lastSha, ok := c.lastProcessedSSSha[key]; ok && lastSha == gitSha {
				fmt.Printf("Ignored rollout: statefulset %s generation changed but fingerprint %s is the same\n", key, gitSha)
				c.lastProcessedSS[key] = ss.Generation
				continue
			}
		}

		// Intent Validation
		source := ss.Annotations["valiant.io/source"]
		if c.requireAnnot {
			if !c.isSourceAllowed(source) {
				continue
			}
		}

		// Completion Validation for StatefulSets:
		// ReadyReplicas == Replicas && UpdatedReplicas == Replicas && CurrentRevision == UpdateRevision
		if ss.Spec.Replicas == nil {
			continue
		}
		desiredReplicas := *ss.Spec.Replicas
		if ss.Status.ReadyReplicas != desiredReplicas ||
			ss.Status.UpdatedReplicas != desiredReplicas ||
			ss.Status.CurrentRevision != ss.Status.UpdateRevision {
			continue
		}

		// Recency check: generation changed and rollout is complete
		if _, ok := c.lastProcessedSS[key]; !ok {
			// First time seeing this StatefulSet — record and skip
			c.lastProcessedSS[key] = ss.Generation
			if gitSha != "" {
				c.lastProcessedSSSha[key] = gitSha
			}
			continue
		}

		// Timing
		rolloutStartTime := ss.CreationTimestamp.Time
		rolloutEndTime := time.Now()

		image, containerName := SelectAppContainer(ss.Spec.Template.Spec.Containers, ss.Annotations)
		previousImage := c.lastProcessedSSImage[key]

		ssMetadata := map[string]string{
			"env":             ss.Namespace,
			"namespace":       ss.Namespace,
			"kind":            "StatefulSet",
			"generation":      fmt.Sprintf("%d", ss.Generation),
			"image":           image,
			"image_tag":       image,
			"container_name":  containerName,
			"intent_source":   source,
			"git_commit_sha":  gitSha,
			"update_revision": ss.Status.UpdateRevision,
		}
		if previousImage != "" && previousImage != image {
			ssMetadata["previous_image_tag"] = previousImage
		}

		eventChan <- domain.ChangeEvent{
			ID:               fmt.Sprintf("k8s-%s-%s-%d", ss.Namespace, ss.Name, ss.Generation),
			Source:           "kubernetes",
			TriggerType:      c.getTriggerType(source),
			IsIntent:         false,
			IsExecution:      true,
			ExecutionID:      fmt.Sprintf("%s-%d", ss.UID, ss.Generation),
			ChangeType:       "statefulset_rollout",
			Timestamp:        rolloutStartTime,
			EndTime:          &rolloutEndTime,
			AffectedServices: []string{ss.Name},
			Summary:          fmt.Sprintf("StatefulSet %s rollout completed via %s", ss.Name, source),
			Metadata:         ssMetadata,
		}
		c.lastProcessedSS[key] = ss.Generation
		c.lastProcessedSSImage[key] = image
		if gitSha != "" {
			c.lastProcessedSSSha[key] = gitSha
		}
	}

	if c.watchConfigMaps {
		c.collectConfigMapChanges(ctx, eventChan, watchedNamespaces, allDepItems, allSSItems)
	}
	if c.watchSecrets {
		c.collectSecretChanges(ctx, eventChan, watchedNamespaces, allDepItems, allSSItems)
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

func isRecentStatefulSetRollout(ss *appsv1.StatefulSet) bool {
	if ss.Spec.Replicas == nil {
		return false
	}
	desired := *ss.Spec.Replicas
	return ss.Status.ReadyReplicas == desired &&
		ss.Status.UpdatedReplicas == desired &&
		ss.Status.CurrentRevision == ss.Status.UpdateRevision
}

func (c *KubernetesCollector) getTriggerType(intentSource string) string {
	if intentSource == "" {
		return "kubernetes-api"
	}
	for _, s := range c.allowedSources {
		if s == intentSource {
			return intentSource
		}
	}
	return "kubernetes-api"
}

func (c *KubernetesCollector) isSourceAllowed(source string) bool {
	if len(c.allowedSources) == 0 {
		return true // If no specific sources are allowed, all are implicitly allowed
	}
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

func findReferencingWorkloads(deps []appsv1.Deployment, ssets []appsv1.StatefulSet, name, kind string) WorkloadReferences {
	refs := WorkloadReferences{}
	for _, d := range deps {
		if referencesResourceInPodSpec(d.Spec.Template.Spec, name, kind) {
			refs.Deployments = append(refs.Deployments, d.Name)
		}
	}
	for _, ss := range ssets {
		if referencesResourceInPodSpec(ss.Spec.Template.Spec, name, kind) {
			refs.StatefulSets = append(refs.StatefulSets, ss.Name)
		}
	}
	return refs
}

func referencesResource(d appsv1.Deployment, name, kind string) bool {
	return referencesResourceInPodSpec(d.Spec.Template.Spec, name, kind)
}

func referencesResourceInPodSpec(podSpec corev1.PodSpec, name, kind string) bool {
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
				fmt.Printf("[valiant] RBAC error: cannot list ConfigMaps cluster-wide. " +
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

func (c *KubernetesCollector) collectConfigMapChanges(ctx context.Context, eventChan chan<- domain.ChangeEvent, watchedNamespaces map[string]bool, deps []appsv1.Deployment, ssets []appsv1.StatefulSet) {
	cmItems := c.listConfigMaps(ctx, watchedNamespaces)

	for _, cm := range cmItems {
		// Namespace filter
		if len(c.namespaces) > 0 && !watchedNamespaces[cm.Namespace] {
			continue
		}

		// Annotation filter
		if c.requireAnnot {
			source := cm.Annotations["valiant.io/source"]
			if !c.isSourceAllowed(source) {
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
		refs := findReferencingWorkloads(deps, ssets, cm.Name, "ConfigMap")
		affectedServices := append(refs.Deployments, refs.StatefulSets...)

		dataKeys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)

		var blastRadius *domain.BlastRadius
		totalWorkloads := len(refs.Deployments) + len(refs.StatefulSets)
		if totalWorkloads > 0 {
			blastRadius = &domain.BlastRadius{
				TotalWorkloads:       totalWorkloads,
				AffectedDeployments:  refs.Deployments,
				AffectedStatefulSets: refs.StatefulSets,
			}
		}

		eventChan <- domain.ChangeEvent{
			ID:               fmt.Sprintf("k8s-cm-%s-%s-%s", cm.Namespace, cm.Name, hash[:8]),
			Source:           "kubernetes",
			TriggerType:      c.getTriggerType(cm.Annotations["valiant.io/source"]),
			IsIntent:         false,
			IsExecution:      true,
			ChangeType:       "configmap_update",
			Timestamp:        time.Now(),
			AffectedServices: affectedServices,
			BlastRadius:      blastRadius,
			Summary:          fmt.Sprintf("ConfigMap %s/%s data changed", cm.Namespace, cm.Name),
			Metadata: map[string]string{
				"env":                      cm.Namespace,
				"namespace":                cm.Namespace,
				"kind":                     "ConfigMap",
				"data_keys":                strings.Join(dataKeys, ","),
				"data_hash":                hash[:8],
				"intent_source":            cm.Annotations["valiant.io/source"],
				"referencing_deployments":  strings.Join(refs.Deployments, ","),
				"referencing_statefulsets": strings.Join(refs.StatefulSets, ","),
				"blast_radius":             fmt.Sprintf("%d", totalWorkloads),
			},
		}
	}
}

func (c *KubernetesCollector) listSecrets(ctx context.Context, watchedNamespaces map[string]bool) []corev1.Secret {
	if len(c.namespaces) == 0 {
		secrets, err := c.clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsForbidden(err) {
				fmt.Printf("[valiant] RBAC error: cannot list Secrets cluster-wide. " +
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

func (c *KubernetesCollector) collectSecretChanges(ctx context.Context, eventChan chan<- domain.ChangeEvent, watchedNamespaces map[string]bool, deps []appsv1.Deployment, ssets []appsv1.StatefulSet) {
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
			source := s.Annotations["valiant.io/source"]
			if !c.isSourceAllowed(source) {
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
		refs := findReferencingWorkloads(deps, ssets, s.Name, "Secret")
		affectedServices := append(refs.Deployments, refs.StatefulSets...)

		dataKeys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)

		var blastRadius *domain.BlastRadius
		totalWorkloads := len(refs.Deployments) + len(refs.StatefulSets)
		if totalWorkloads > 0 {
			blastRadius = &domain.BlastRadius{
				TotalWorkloads:       totalWorkloads,
				AffectedDeployments:  refs.Deployments,
				AffectedStatefulSets: refs.StatefulSets,
			}
		}

		eventChan <- domain.ChangeEvent{
			ID:               fmt.Sprintf("k8s-secret-%s-%s-%s", s.Namespace, s.Name, hash[:8]),
			Source:           "kubernetes",
			TriggerType:      c.getTriggerType(s.Annotations["valiant.io/source"]),
			IsIntent:         false,
			IsExecution:      true,
			ChangeType:       "secret_update",
			Timestamp:        time.Now(),
			AffectedServices: affectedServices,
			BlastRadius:      blastRadius,
			Summary:          fmt.Sprintf("Secret %s/%s data changed", s.Namespace, s.Name),
			Metadata: map[string]string{
				"env":                      s.Namespace,
				"namespace":                s.Namespace,
				"kind":                     "Secret",
				"data_keys":                strings.Join(dataKeys, ","),
				"intent_source":            s.Annotations["valiant.io/source"],
				"referencing_deployments":  strings.Join(refs.Deployments, ","),
				"referencing_statefulsets": strings.Join(refs.StatefulSets, ","),
				"blast_radius":             fmt.Sprintf("%d", totalWorkloads),
			},
		}
	}
}

// knownSidecars is the set of container names recognized as sidecars and
// excluded when selecting the application container image.
var knownSidecars = map[string]bool{
	"istio-proxy":   true,
	"envoy":         true,
	"linkerd-proxy": true,
	"vault-agent":   true,
}

// SelectAppContainer picks the most likely application container from a pod spec.
// Priority: 1) explicit valiant.io/container annotation, 2) first non-sidecar, 3) first container.
// Returns the image string and the container name.
func SelectAppContainer(containers []corev1.Container, annotations map[string]string) (image string, containerName string) {
	// Explicit override via annotation
	if name, ok := annotations["valiant.io/container"]; ok {
		for _, c := range containers {
			if c.Name == name {
				return c.Image, c.Name
			}
		}
	}
	// Filter known sidecars
	for _, c := range containers {
		if !knownSidecars[c.Name] {
			return c.Image, c.Name
		}
	}
	// Fallback to first container
	if len(containers) > 0 {
		return containers[0].Image, containers[0].Name
	}
	return "", ""
}

func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}
