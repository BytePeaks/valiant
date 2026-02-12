package configmaps

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

func TestConfigMapChangeDetection(t *testing.T) {
	// ARRANGE
	namespace := "default"
	cmName := "test-config"
	depName := "test-app"

	// Initial ConfigMap
	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})

	// Deployment referencing the ConfigMap
	dep := shared.SampleDeployment(depName, namespace, "nginx:1.21", "abcdef")
	// Modify deployment to reference the configmap via a volume
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmName,
					},
				},
			},
		},
	}
	// Add deployment to fakeClient, as it's needed for affected services resolution
	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM, dep}...)

	// Configure the collector to watch ConfigMaps
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{namespace} // Watch this namespace

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT - Initial collection to establish baseline hash
	collector.CollectAndSend(context.Background(), eventChan)

	// Ensure no event for first seen ConfigMap
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected initial change event: %+v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: no event on first seen
	}

	// Update the ConfigMap in the fake client
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key2"] = "value2"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	// ACT - Second collection after update
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT - Expect a configmap_update event
	select {
	case event := <-eventChan:
		shared.AssertChangeEvent(t, event,
			"", // ID is dynamic (hash based), so don't assert specific ID
			"kubernetes",
			"configmap_update",
			"ConfigMap default/test-config data changed")
		shared.AssertChangeEventMetadata(t, event, "namespace", namespace)
		shared.AssertChangeEventAffectedServices(t, event, []string{depName})
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for configmap update event")
	}
	assert.Empty(t, eventChan, "Expected only one event") // Ensure no extra events
}

func TestConfigMapChangeIncludesStatefulSetReferences(t *testing.T) {
	// ARRANGE — ConfigMap referenced by both a Deployment and a StatefulSet
	namespace := "default"
	cmName := "shared-config"

	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})

	// Deployment referencing ConfigMap via volume
	dep := shared.SampleDeployment("web-app", namespace, "nginx:1.21", "sha-1")
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		},
	}

	// StatefulSet referencing same ConfigMap via envFrom
	ss := shared.SampleStatefulSet("db-app", namespace, "postgres:15", "sha-2")
	ss.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM, dep, ss}...)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Baseline collection
	coll.CollectAndSend(context.Background(), eventChan)

	// Update ConfigMap
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key2"] = "new-value"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT — AffectedServices should include both Deployment and StatefulSet names
	var configEvent *domain.ChangeEvent
	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-eventChan:
			if event.ChangeType == "configmap_update" {
				configEvent = &event
			}
		case <-timeout:
			goto done
		}
	}
done:

	require.NotNil(t, configEvent, "Expected a configmap_update event")
	assert.Contains(t, configEvent.AffectedServices, "web-app", "Should include referencing Deployment")
	assert.Contains(t, configEvent.AffectedServices, "db-app", "Should include referencing StatefulSet")
}

func TestConfigMapNoChangeEventIfNoDataChange(t *testing.T) {
	// ARRANGE
	namespace := "default"
	cmName := "test-config"

	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})
	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM}...)

	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
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
