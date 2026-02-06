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

func TestAnnotationFiltering_NoAnnotation(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	fakeClient := shared.NewFakeKubeClient()

	// Create a deployment without the valiant.io/source annotation
	dep := shared.SampleDeployment("test-app", "default", "nginx:1.21", "abcdef")
	delete(dep.Annotations, "valiant.io/source") // Remove the annotation
	shared.SetDeploymentReady(dep)

	_, err = fakeClient.AppsV1().Deployments(dep.Namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Configure the collector to require the annotation
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd"}

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	// Expect no event because the annotation is missing
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected change event: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// This is the expected outcome
	}
}

func TestAnnotationFiltering_UnallowedSource(t *testing.T) {
	// ARRANGE
	db, schemaName, err := shared.SetupTestDB()
	require.NoError(t, err)
	defer shared.CleanupTestDB(db, schemaName)

	fakeClient := shared.NewFakeKubeClient()

	// Create a deployment with an unallowed source annotation
	dep := shared.SampleDeployment("test-app", "default", "nginx:1.21", "abcdef")
	dep.Annotations["valiant.io/source"] = "unallowed-source"
	shared.SetDeploymentReady(dep)

	_, err = fakeClient.AppsV1().Deployments(dep.Namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"cicd", "argocd"} // "unallowed-source" is not in this list

	collector, err := collector.NewKubernetesCollector(cfg, fakeClient)
	require.NoError(t, err)

	eventChan := make(chan domain.ChangeEvent, 1)

	// ACT
	collector.CollectAndSend(context.Background(), eventChan)

	// ASSERT
	select {
	case event := <-eventChan:
		t.Fatalf("Received unexpected change event from unallowed source: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// This is the expected outcome
	}
}
