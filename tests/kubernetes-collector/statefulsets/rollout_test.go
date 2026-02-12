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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatefulSetRolloutDetection(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	fakeClient := shared.NewFakeKubeClient()

	ssName := "test-db"
	namespace := "default"
	image := "postgres:15"
	sha := "ss-sha-123"
	ss := shared.SampleStatefulSet(ssName, namespace, image, sha)
	shared.SetStatefulSetReady(ss)

	_, err = fakeClient.AppsV1().StatefulSets(namespace).Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// ACT - First call seeds the generation tracker, second call detects change
	coll.CollectAndSend(context.Background(), eventChan)

	// For StatefulSets, first call records the generation; update generation to trigger detection
	ss.Generation = 2
	ss.Annotations["valiant.io/git-sha"] = "ss-sha-456"
	shared.SetStatefulSetReady(ss)
	_, err = fakeClient.AppsV1().StatefulSets(namespace).Update(context.Background(), ss, metav1.UpdateOptions{})
	require.NoError(t, err)

	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	select {
	case event := <-eventChan:
		shared.AssertChangeEvent(t, event,
			"k8s-default-test-db-2",
			"kubernetes",
			"statefulset_rollout",
			"StatefulSet test-db rollout completed via cicd")
		shared.AssertChangeEventMetadata(t, event, "namespace", namespace)
		shared.AssertChangeEventMetadata(t, event, "kind", "StatefulSet")
		shared.AssertChangeEventMetadata(t, event, "image", image)
		shared.AssertChangeEventAffectedServices(t, event, []string{ssName})
		require.NotNil(t, event.EndTime, "Expected EndTime to be populated")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for statefulset rollout event")
	}
}

func TestStatefulSetNotReady_NoEvent(t *testing.T) {
	// ARRANGE
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "default", "postgres:15", "sha-123")
	shared.SetStatefulSetNotReady(ss) // Not complete — should not emit

	_, err := fakeClient.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// ACT
	coll.CollectAndSend(context.Background(), eventChan)
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT - No event because StatefulSet is not ready
	select {
	case event := <-eventChan:
		// Filter out deployment events, only fail on statefulset events
		if event.ChangeType == "statefulset_rollout" {
			t.Fatalf("Unexpected statefulset rollout event: %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestStatefulSetDeduplication(t *testing.T) {
	// ARRANGE
	fakeClient := shared.NewFakeKubeClient()

	ss := shared.SampleStatefulSet("test-db", "default", "postgres:15", "sha-123")
	shared.SetStatefulSetReady(ss)

	_, err := fakeClient.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	coll, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 5)

	// ACT - First call seeds, second should NOT re-emit for same generation
	coll.CollectAndSend(context.Background(), eventChan)
	coll.CollectAndSend(context.Background(), eventChan)

	// ASSERT - No duplicate events (first call is seeding, second is same generation)
	ssEvents := 0
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case event := <-eventChan:
			if event.ChangeType == "statefulset_rollout" {
				ssEvents++
			}
		case <-timeout:
			assert.Equal(t, 0, ssEvents, "Expected no statefulset events (same generation on both calls)")
			return
		}
	}
}
