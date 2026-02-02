package collector_test

import (
	"context"
	"fmt"
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

	go coll.Start(ctx, eventChan)

	select {
	case event := <-eventChan:
		if event.AffectedServices[0] != "intentional-app" {
			t.Errorf("expected event for intentional-app, got %s", event.AffectedServices[0])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event")
	}

	select {
	case event := <-eventChan:
		t.Errorf("unexpected second event: %s", event.Summary)
	default:
	}
}

func newTestDeployment(name string, generation int64, sha string, available bool) *appsv1.Deployment {
	annotations := map[string]string{
		"valiant.io/source": "argocd",
	}
	if sha != "" {
		annotations["valiant.io/git-sha"] = sha
	}

	conditions := []appsv1.DeploymentCondition{}
	if available {
		conditions = append(conditions, appsv1.DeploymentCondition{
			Type:           "Available",
			Status:         "True",
			LastUpdateTime: metav1.Now(),
		})
		conditions = append(conditions, appsv1.DeploymentCondition{
			Type:   "Progressing",
			Reason: "NewReplicaSetAvailable",
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: annotations,
			Generation:  generation,
			UID:         "test-uid",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: conditions,
		},
	}
}

func newTestDeploymentInNamespace(name, namespace string, generation int64, sha string, available bool) *appsv1.Deployment {
	annotations := map[string]string{
		"valiant.io/source": "argocd",
	}
	if sha != "" {
		annotations["valiant.io/git-sha"] = sha
	}

	conditions := []appsv1.DeploymentCondition{}
	if available {
		conditions = append(conditions, appsv1.DeploymentCondition{
			Type:           "Available",
			Status:         "True",
			LastUpdateTime: metav1.Now(),
		})
		conditions = append(conditions, appsv1.DeploymentCondition{
			Type:   "Progressing",
			Reason: "NewReplicaSetAvailable",
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
			Generation:  generation,
			UID:         "test-uid",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: conditions,
		},
	}
}

func TestKubernetesCollector_FingerprintDeduplication(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"argocd"}

	client := fake.NewSimpleClientset()
	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	expectEvent := func(shouldExist bool, expectedGen int64, step string) {
		t.Helper()
		select {
		case event := <-eventChan:
			if !shouldExist {
				t.Fatalf("[%s] unexpected event for generation %s", step, event.Metadata["generation"])
			}
			genStr := event.Metadata["generation"]
			if genStr != fmt.Sprintf("%d", expectedGen) {
				t.Errorf("[%s] expected event for generation %d, got %s", step, expectedGen, genStr)
			}
		case <-time.After(100 * time.Millisecond):
			if shouldExist {
				t.Fatalf("[%s] timeout waiting for event for generation %d", step, expectedGen)
			}
		}
	}

	// 2. Initial deploy (gen 1, sha A) -> SHOULD collect
	dep := newTestDeployment("app-a", 1, "sha-a", true)
	if _, err := client.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	expectEvent(true, 1, "Initial Deploy")

	// 3. Metadata change (gen 2, sha A) -> SHOULD IGNORE
	dep.Generation = 2
	dep.ResourceVersion = "2" 
	if _, err := client.AppsV1().Deployments("default").Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	expectEvent(false, 2, "Metadata Change")

	// 4. New code (gen 3, sha B) -> SHOULD collect
	dep.Generation = 3
	dep.ResourceVersion = "3"
	dep.Annotations["valiant.io/git-sha"] = "sha-b"
	if _, err := client.AppsV1().Deployments("default").Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	expectEvent(true, 3, "New Code")

	// 5. Rollback (gen 4, sha A) -> SHOULD collect
	dep.Generation = 4
	dep.ResourceVersion = "4"
	dep.Annotations["valiant.io/git-sha"] = "sha-a"
	if _, err := client.AppsV1().Deployments("default").Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	expectEvent(true, 4, "Rollback")

	// 6. No change (gen 4, sha A) -> SHOULD IGNORE (by generation check)
	dep.ResourceVersion = "5"
	if _, err := client.AppsV1().Deployments("default").Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	expectEvent(false, 4, "No Change")
}

func TestKubernetesCollector_NamespaceFilter(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.Namespaces = []string{"watched-ns"} // Only watch this namespace
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"argocd"}

	client := fake.NewSimpleClientset()
	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// Deploy in a watched namespace -> SHOULD collect
	depWatched := newTestDeploymentInNamespace("app-watched", "watched-ns", 1, "sha-w", true)
	if _, err := client.AppsV1().Deployments("watched-ns").Create(ctx, depWatched, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		if event.AffectedServices[0] != "app-watched" {
			t.Errorf("expected event for app-watched, got %s", event.AffectedServices[0])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event from watched namespace")
	}

	// Deploy in an unwatched namespace -> SHOULD NOT collect
	depUnwatched := newTestDeploymentInNamespace("app-unwatched", "unwatched-ns", 1, "sha-u", true)
	if _, err := client.AppsV1().Deployments("unwatched-ns").Create(ctx, depUnwatched, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		t.Errorf("unexpected event from unwatched namespace: %s", event.Summary)
	case <-time.After(100 * time.Millisecond):
		// Expected timeout, no event should be received
	}
}