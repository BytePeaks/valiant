package collector_test

import (
	"context"
	"testing"
	"time"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/domain"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesCollector_AuditorLogic(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"argocd"}

	// 1. Setup Fake Clientset with 2 Deployments: one valid, one manual
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "intentional-app",
				Namespace: "default",
				Annotations: map[string]string{
					"valiant.io/source": "argocd",
				},
				Generation: 1,
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable", LastTransitionTime: metav1.Now()},
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "manual-app",
				Namespace:  "default",
				Generation: 1,
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
				},
			},
		},
	)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	eventChan := make(chan domain.ChangeEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// In a test, we can call the internal method directly if we make it public or 
	// just run Start and wait for events.
	// Since I made Start public, I'll run it in a goroutine.
	go coll.Start(ctx, eventChan)

	// Wait for one event (intentional-app)
	select {
	case event := <-eventChan:
		if event.AffectedServices[0] != "intentional-app" {
			t.Errorf("expected event for intentional-app, got %s", event.AffectedServices[0])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event")
	}

	// Verify no more events (manual-app should be ignored)
	select {
	case event := <-eventChan:
		t.Errorf("unexpected second event: %s", event.Summary)
	default:
		// Success: no more events
	}
}

// More advanced K8s tests would require mocking the clientset entirely
// which NewKubernetesCollector doesn't currently support (it creates its own).
// Refactoring NewKubernetesCollector to accept a clientset interface would improve testability.
