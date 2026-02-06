package deployments

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

func TestNamespaceFiltering(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	fakeClient := shared.NewFakeKubeClient()

	// Create a deployment in a namespace that will NOT be watched
	unwatchedNamespace := "unwatched-ns"
	dep := shared.SampleDeployment("test-app", unwatchedNamespace, "nginx:1.21", "abcdef")
	shared.SetDeploymentReady(dep)

	_, err = fakeClient.AppsV1().Deployments(dep.Namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Configure the collector to watch a different namespace
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}
	cfg.Kubernetes.Namespaces = []string{"watched-ns"} // Collector watches this namespace only

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	// Expect no event because the deployment is in an unwatched namespace
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected change event from unwatched namespace: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// This is the expected outcome
	}
}
