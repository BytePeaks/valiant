package metadata

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

func TestDeploymentMetadata_AllExpectedKeys(t *testing.T) {
	fakeClient := shared.NewFakeKubeClient()
	dep := shared.SampleDeployment("test-app", "default", "myregistry/myapp:v1.5.0", "abc123")
	shared.SetDeploymentReady(dep)

	_, err := fakeClient.AppsV1().Deployments(dep.Namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case event := <-eventChan:
		expectedKeys := []string{"namespace", "kind", "generation", "image", "intent_source", "git_commit_sha", "rollout_start", "rollout_end"}
		for _, key := range expectedKeys {
			assert.Contains(t, event.Metadata, key, "Missing metadata key: %s", key)
		}
		assert.Equal(t, "default", event.Metadata["namespace"])
		assert.Equal(t, "Deployment", event.Metadata["kind"])
		assert.Equal(t, "1", event.Metadata["generation"])
		assert.Equal(t, "myregistry/myapp:v1.5.0", event.Metadata["image"])
		assert.Equal(t, "cicd", event.Metadata["intent_source"])
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for deployment event")
	}
}

func TestConfigMapMetadata_DataKeysPresent(t *testing.T) {
	namespace := "default"
	cmName := "app-config"
	depName := "test-app"

	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{
		"database_url": "postgres://localhost/db",
		"log_level":    "info",
		"cache_ttl":    "300",
	})

	dep := shared.SampleDeployment(depName, namespace, "nginx:1.21", "sha1")
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
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

	eventChan := make(chan domain.ChangeEvent, 1)

	// First collection to establish baseline
	coll.CollectAndSend(context.Background(), eventChan)

	// Update ConfigMap
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["log_level"] = "debug"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Second collection to detect change
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case event := <-eventChan:
		assert.Contains(t, event.Metadata, "data_keys", "ConfigMap event should have data_keys metadata")
		// data_keys should contain sorted keys
		assert.Contains(t, event.Metadata["data_keys"], "cache_ttl")
		assert.Contains(t, event.Metadata["data_keys"], "database_url")
		assert.Contains(t, event.Metadata["data_keys"], "log_level")
		assert.Contains(t, event.Metadata, "data_hash")
		assert.Contains(t, event.Metadata, "namespace")
		assert.Equal(t, "ConfigMap", event.Metadata["kind"])
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for configmap update event")
	}
}

func TestImageTagExtraction_VariousFormats(t *testing.T) {
	testCases := []struct {
		name          string
		image         string
		expectedImage string
	}{
		{"simple_tag", "nginx:1.21", "nginx:1.21"},
		{"registry_with_tag", "myregistry.io/myapp:v2.0.0", "myregistry.io/myapp:v2.0.0"},
		{"digest", "nginx@sha256:abcdef1234567890", "nginx@sha256:abcdef1234567890"},
		{"latest", "nginx:latest", "nginx:latest"},
		{"no_tag", "nginx", "nginx"},
		{"nested_registry", "gcr.io/my-project/my-app:1.0", "gcr.io/my-project/my-app:1.0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := shared.NewFakeKubeClient()
			dep := shared.SampleDeployment("test-app", "default", tc.image, "sha123")
			shared.SetDeploymentReady(dep)

			_, err := fakeClient.AppsV1().Deployments(dep.Namespace).Create(
				context.Background(), dep, metav1.CreateOptions{})
			require.NoError(t, err)

			cfg := config.Config{}
			cfg.Kubernetes.RequireAnnotation = true
			cfg.Kubernetes.AllowedSources = []string{"cicd"}

			coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
			require.NoError(t, err)

			eventChan := make(chan domain.ChangeEvent, 1)
			coll.CollectAndSend(context.Background(), eventChan)

			select {
			case event := <-eventChan:
				assert.Equal(t, tc.expectedImage, event.Metadata["image"],
					"Image metadata should preserve full image reference")
			case <-time.After(1 * time.Second):
				t.Fatalf("Timeout waiting for event with image %s", tc.image)
			}
		})
	}
}
