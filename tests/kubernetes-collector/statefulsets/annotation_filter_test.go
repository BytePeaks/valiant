package statefulsets

import (
	"context"
	"testing"
	"time"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/tests/kubernetes-collector/shared"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestStatefulSetAnnotationFilter_Excluded(t *testing.T) {
	// StatefulSet without valiant.io/source annotation should be skipped when RequireAnnotation is true
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "default", "postgres:15", "sha-123")
	// Remove the annotation
	ss.Annotations = map[string]string{}
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Seed + trigger
	coll.CollectAndSend(context.Background(), eventChan)
	ss.Generation = 2
	shared.SetStatefulSetReady(ss)
	_, err = fakeClient.AppsV1().StatefulSets("default").Update(context.Background(), ss, metav1.UpdateOptions{})
	require.NoError(t, err)
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT - No event due to annotation filter
	select {
	case event := <-eventChan:
		if event.ChangeType == "statefulset_rollout" {
			t.Fatalf("Unexpected statefulset event (should be filtered by annotation): %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestStatefulSetAnnotationFilter_WrongSource(t *testing.T) {
	// StatefulSet with wrong source annotation should be skipped
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "default", "postgres:15", "sha-123")
	ss.Annotations = map[string]string{
		"valiant.io/source": "unknown-source",
	}
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	coll.CollectAndSend(context.Background(), eventChan)
	ss.Generation = 2
	ss.Annotations["valiant.io/source"] = "unknown-source"
	shared.SetStatefulSetReady(ss)
	fakeClient.AppsV1().StatefulSets("default").Update(context.Background(), ss, metav1.UpdateOptions{})
	coll.CollectAndSend(context.Background(), eventChan)

	select {
	case event := <-eventChan:
		if event.ChangeType == "statefulset_rollout" {
			t.Fatalf("Unexpected statefulset event (wrong source): %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestStatefulSetAnnotationFilter_NotRequired(t *testing.T) {
	// When RequireAnnotation is false, StatefulSets without annotations should still be collected
	fakeClient := shared.NewFakeKubeClient()

	ss := &appsv1.StatefulSet{}
	*ss = *shared.SampleStatefulSet("test-db", "default", "postgres:15", "sha-123")
	ss.Annotations = map[string]string{} // No valiant annotations
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = false // Not required

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Seed
	coll.CollectAndSend(context.Background(), eventChan)

	// Update generation
	ss.Generation = 2
	shared.SetStatefulSetReady(ss)
	_, err = fakeClient.AppsV1().StatefulSets("default").Update(context.Background(), ss, metav1.UpdateOptions{})
	require.NoError(t, err)

	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT - Should receive event even without annotation
	select {
	case event := <-eventChan:
		if event.ChangeType == "statefulset_rollout" {
			shared.AssertChangeEvent(t, event, "", "kubernetes", "statefulset_rollout", "StatefulSet test-db rollout completed")
			return
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Expected statefulset event when RequireAnnotation is false")
	}
}
