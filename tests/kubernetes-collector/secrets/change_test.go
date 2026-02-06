package secrets

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

func TestSecretChangeDetection(t *testing.T) {
	// ARRANGE
	namespace := "default"
	secretName := "test-secret"
	depName := "test-app"

	// Initial Secret
	initialSecret := shared.SampleSecret(secretName, namespace, map[string][]byte{"key1": []byte("value1")})

	// Deployment referencing the Secret
	dep := shared.SampleDeployment(depName, namespace, "nginx:1.21", "abcdef")
	// Modify deployment to reference the secret via an envFrom
	dep.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:  "nginx",
			Image: "nginx:1.21",
			EnvFrom: []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}
	// Add deployment to fakeClient, as it's needed for affected services resolution
	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialSecret, dep}...)

	// Configure the collector to watch Secrets
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.WatchSecrets = true
	cfg.Kubernetes.Namespaces = []string{namespace} // Watch this namespace

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT - Initial collection to establish baseline hash
	collector.CollectAndSend(context.Background(), eventChan)

	// Ensure no event for first seen Secret
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected initial change event: %+v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: no event on first seen
	}

	// Update the Secret data in the fake client
	updatedSecret := initialSecret.DeepCopy()
	updatedSecret.Data["key2"] = []byte("value2")
	_, err = fakeClient.CoreV1().Secrets(namespace).Update(context.Background(), updatedSecret, metav1.UpdateOptions{})
	require.NoError(t, err)

	// ACT - Second collection after update
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT - Expect a secret_update event
	select {
	case event := <-eventChan:
		shared.AssertChangeEvent(t, event,
			"", // ID is dynamic (hash based), so don't assert specific ID
			"kubernetes",
			"secret_update",
			"Secret default/test-secret data changed")
		shared.AssertChangeEventMetadata(t, event, "namespace", namespace)
		shared.AssertChangeEventAffectedServices(t, event, []string{depName})
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for secret update event")
	}
	assert.Empty(t, eventChan, "Expected only one event") // Ensure no extra events
}

func TestSecretNoChangeEventIfNoDataChange(t *testing.T) {
	// ARRANGE
	namespace := "default"
	secretName := "test-secret"

	initialSecret := shared.SampleSecret(secretName, namespace, map[string][]byte{"key1": []byte("value1")})
	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialSecret}...)

	cfg := config.Config{}
	cfg.Kubernetes.WatchSecrets = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT - Initial collection
	collector.CollectAndSend(context.Background(), eventChan)
	// ACT - Second collection without any change
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT - Expect no event
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected change event: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}
