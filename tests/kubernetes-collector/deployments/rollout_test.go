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

func TestDeploymentRolloutDetection(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	// Create a fake Kubernetes client
	fakeClient := shared.NewFakeKubeClient()

	// Create a sample Deployment
	deploymentName := "test-app"
	namespace := "default"
	image := "nginx:1.21"
	sha := "abcdef123456"
	dep := shared.SampleDeployment(deploymentName, namespace, image, sha)
	shared.SetDeploymentReady(dep) // Ensure it's ready for collection

	// Add the deployment to the fake client
	_, err = fakeClient.AppsV1().Deployments(dep.Namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create a config for the collector
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	// Create the Kubernetes collector
	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	// Create an event channel to receive collected events
	eventChan := make(chan domain.ChangeEvent, 1) // Buffered channel for a single event

	// ACT
	// Manually call CollectAndSend to trigger event collection
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	select {
	case event := <-eventChan:
		shared.AssertChangeEvent(t, event,
			"k8s-default-test-app-1",
			"kubernetes",
			"deployment_rollout",
			"Deployment test-app rollout completed via cicd")
		shared.AssertChangeEventMetadata(t, event, "namespace", namespace)
		shared.AssertChangeEventMetadata(t, event, "image", image)
		shared.AssertChangeEventMetadata(t, event, "git_commit_sha", sha)
		shared.AssertChangeEventAffectedServices(t, event, []string{deploymentName})
		require.NotNil(t, event.EndTime, "Expected EndTime to be populated")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for change event")
	}
}
