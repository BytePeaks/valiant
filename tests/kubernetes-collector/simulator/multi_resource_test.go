package simulator

import (
	"context"
	"testing"
	"time"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/tests/kubernetes-collector/shared"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentAndConfigMapChange_SameCycle(t *testing.T) {
	namespace := "default"
	depName := "test-app"
	cmName := "test-config"

	// Initial ConfigMap
	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})

	// Deployment referencing ConfigMap
	dep := shared.SampleDeployment(depName, namespace, "nginx:1.21", "sha-init")
	shared.SetDeploymentReady(dep)
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM, dep}...)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	// Use buffered channel to capture multiple events
	eventChan := make(chan domain.ChangeEvent, 5)

	// First collection: deployment rollout detected + ConfigMap baseline established
	coll.CollectAndSend(context.Background(), eventChan)

	// Drain the deployment event from first collection
	var deployEvent *domain.ChangeEvent
	select {
	case ev := <-eventChan:
		deployEvent = &ev
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial deployment event")
	}

	assert.Equal(t, "deployment_rollout", deployEvent.ChangeType)

	// Now update the ConfigMap
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key2"] = "value2"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Second collection: should detect ConfigMap change (deployment won't re-fire since gen unchanged)
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case ev := <-eventChan:
		assert.Equal(t, "configmap_update", ev.ChangeType, "Expected configmap_update event")
		assert.Contains(t, ev.AffectedServices, depName,
			"ConfigMap change should list referencing deployment as affected service")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for configmap update event")
	}
}

func TestMultipleDeployments_SameConfigMap(t *testing.T) {
	namespace := "default"
	cmName := "shared-config"

	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key": "val"})

	// Two deployments referencing the same ConfigMap
	dep1 := shared.SampleDeployment("app-1", namespace, "nginx:1.21", "sha1")
	dep1.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		},
	}

	dep2 := shared.SampleDeployment("app-2", namespace, "nginx:1.22", "sha2")
	dep2.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM, dep1, dep2}...)

	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// First collection to establish baseline
	coll.CollectAndSend(context.Background(), eventChan)

	// Drain any deployment events
	drainEvents(eventChan)

	// Update ConfigMap
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key"] = "new-val"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Second collection
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case event := <-eventChan:
		assert.Equal(t, "configmap_update", event.ChangeType)
		// Both deployments reference this ConfigMap
		assert.ElementsMatch(t, []string{"app-1", "app-2"}, event.AffectedServices,
			"Both deployments referencing the ConfigMap should be listed as affected")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for configmap update event")
	}
}

func TestRollback_GenerationDecreases(t *testing.T) {
	// Rollbacks in K8s increase the generation, not decrease it.
	// But a new rollout after a rollback should still be detected.
	namespace := "default"
	depName := "rollback-app"

	dep := shared.SampleDeployment(depName, namespace, "nginx:1.21", "sha-v1")
	shared.SetDeploymentReady(dep)

	fakeClient := shared.NewFakeKubeClient()
	_, err := fakeClient.AppsV1().Deployments(namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// First collection: detect initial rollout
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case ev := <-eventChan:
		assert.Equal(t, "deployment_rollout", ev.ChangeType)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial rollout")
	}

	// Simulate rollback: generation increases, image changes back
	dep.Generation = 2
	dep.Spec.Template.Spec.Containers[0].Image = "nginx:1.20" // Rolled back to older version
	dep.Annotations["valiant.io/git-sha"] = "sha-v0" // Different SHA
	shared.SetDeploymentReady(dep)

	_, err = fakeClient.AppsV1().Deployments(namespace).Update(context.Background(), dep, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Second collection: detect rollback as new rollout
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case ev := <-eventChan:
		assert.Equal(t, "deployment_rollout", ev.ChangeType)
		assert.Equal(t, "nginx:1.20", ev.Metadata["image"],
			"Rollback should show the rolled-back image version")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for rollback event")
	}
}

// drainEvents reads all available events from the channel without blocking
func drainEvents(ch chan domain.ChangeEvent) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
