package blast_radius

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

func TestBlastRadius_ConfigMapReferencedByMultipleWorkloads(t *testing.T) {
	// ARRANGE
	namespace := "default"
	cmName := "shared-config"

	// ConfigMap referenced by both a Deployment and a StatefulSet
	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})

	// Deployment referencing the ConfigMap via volume
	dep := shared.SampleDeployment("web-app", namespace, "nginx:1.21", "sha-1")
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

	// StatefulSet referencing the same ConfigMap via envFrom
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

	// ACT - Initial collection to establish baseline hash
	coll.CollectAndSend(context.Background(), eventChan)

	// Update the ConfigMap
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key2"] = "value2"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Second collection after update
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT
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

	// Should have both Deployment and StatefulSet in affected services
	assert.Contains(t, configEvent.AffectedServices, "web-app", "Should include referencing Deployment")
	assert.Contains(t, configEvent.AffectedServices, "db-app", "Should include referencing StatefulSet")

	// BlastRadius should be populated
	require.NotNil(t, configEvent.BlastRadius, "BlastRadius should be set")
	assert.Equal(t, 2, configEvent.BlastRadius.TotalWorkloads)
	assert.Contains(t, configEvent.BlastRadius.AffectedDeployments, "web-app")
	assert.Contains(t, configEvent.BlastRadius.AffectedStatefulSets, "db-app")

	// Metadata should include both referencing types
	assert.Contains(t, configEvent.Metadata["referencing_deployments"], "web-app")
	assert.Contains(t, configEvent.Metadata["referencing_statefulsets"], "db-app")
	assert.Equal(t, "2", configEvent.Metadata["blast_radius"])
}

func TestBlastRadius_SecretReferencedByStatefulSet(t *testing.T) {
	// ARRANGE
	namespace := "default"
	secretName := "db-credentials"

	initialSecret := shared.SampleSecret(secretName, namespace, map[string][]byte{"password": []byte("secret1")})

	// StatefulSet referencing the secret via volume
	ss := shared.SampleStatefulSet("db-app", namespace, "postgres:15", "sha-1")
	ss.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "secret-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialSecret, ss}...)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.WatchSecrets = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// ACT - Baseline
	coll.CollectAndSend(context.Background(), eventChan)

	// Update secret
	updatedSecret := initialSecret.DeepCopy()
	updatedSecret.Data["password"] = []byte("secret2")
	_, err = fakeClient.CoreV1().Secrets(namespace).Update(context.Background(), updatedSecret, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Collect after update
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	var secretEvent *domain.ChangeEvent
	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-eventChan:
			if event.ChangeType == "secret_update" {
				secretEvent = &event
			}
		case <-timeout:
			goto done2
		}
	}
done2:

	require.NotNil(t, secretEvent, "Expected a secret_update event")

	assert.Contains(t, secretEvent.AffectedServices, "db-app")

	require.NotNil(t, secretEvent.BlastRadius, "BlastRadius should be set")
	assert.Equal(t, 1, secretEvent.BlastRadius.TotalWorkloads)
	assert.Contains(t, secretEvent.BlastRadius.AffectedStatefulSets, "db-app")
	assert.Empty(t, secretEvent.BlastRadius.AffectedDeployments, "No deployments reference this secret")
}

func TestBlastRadius_NoReferences_NilBlastRadius(t *testing.T) {
	// ConfigMap not referenced by any workload should have nil BlastRadius
	namespace := "default"
	cmName := "orphan-config"

	initialCM := shared.SampleConfigMap(cmName, namespace, map[string]string{"key1": "value1"})
	// No deployments or statefulsets reference this ConfigMap
	fakeClient := fake.NewSimpleClientset([]runtime.Object{initialCM}...)

	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{namespace}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Baseline
	coll.CollectAndSend(context.Background(), eventChan)

	// Update
	updatedCM := initialCM.DeepCopy()
	updatedCM.Data["key2"] = "value2"
	_, err = fakeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), updatedCM, metav1.UpdateOptions{})
	require.NoError(t, err)

	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	var configEvent *domain.ChangeEvent
	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-eventChan:
			if event.ChangeType == "configmap_update" {
				configEvent = &event
			}
		case <-timeout:
			goto done3
		}
	}
done3:

	require.NotNil(t, configEvent, "Expected a configmap_update event")
	assert.Nil(t, configEvent.BlastRadius, "BlastRadius should be nil when no workloads reference the ConfigMap")
	assert.Empty(t, configEvent.AffectedServices, "No services should be affected")
}
