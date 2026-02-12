package statefulsets

import (
	"context"
	"testing"
	"time"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/tests/kubernetes-collector/shared"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestStatefulSetNamespaceFilter_Excluded(t *testing.T) {
	// StatefulSet in a non-watched namespace should be skipped
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "other-namespace", "postgres:15", "sha-123")
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("other-namespace").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.Namespaces = []string{"default"} // Only watch "default"

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Seed + update
	coll.CollectAndSend(context.Background(), eventChan)
	ss.Generation = 2
	shared.SetStatefulSetReady(ss)
	_, err = fakeClient.AppsV1().StatefulSets("other-namespace").Update(context.Background(), ss, metav1.UpdateOptions{})
	require.NoError(t, err)
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT - No event for namespace not in watched list
	select {
	case event := <-eventChan:
		if event.ChangeType == "statefulset_rollout" {
			t.Fatalf("Unexpected statefulset event from non-watched namespace: %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestStatefulSetNamespaceFilter_Included(t *testing.T) {
	// StatefulSet in a watched namespace should be collected
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "production", "postgres:15", "sha-123")
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("production").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.Namespaces = []string{"production"} // Watch production

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// Seed
	coll.CollectAndSend(context.Background(), eventChan)

	// Update generation to trigger event
	ss.Generation = 2
	ss.Annotations["valiant.io/git-sha"] = "sha-456"
	shared.SetStatefulSetReady(ss)
	_, err = fakeClient.AppsV1().StatefulSets("production").Update(context.Background(), ss, metav1.UpdateOptions{})
	require.NoError(t, err)

	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	select {
	case event := <-eventChan:
		if event.ChangeType == "statefulset_rollout" {
			shared.AssertChangeEvent(t, event, "", "kubernetes", "statefulset_rollout", "StatefulSet test-db rollout completed")
			return
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Expected statefulset event from watched namespace")
	}
}
