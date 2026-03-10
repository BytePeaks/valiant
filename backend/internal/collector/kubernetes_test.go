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
	corev1 "k8s.io/api/core/v1"
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

// --- Test helpers for ConfigMap/Secret tests ---

func newTestConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"valiant.io/source": "argocd",
			},
		},
		Data: data,
	}
}

func newTestSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"valiant.io/source": "argocd",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}

func newTestDeploymentWithConfigMapRef(name, namespace, cmName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
			UID:        "test-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newTestDeploymentWithSecretRef(name, namespace, secretName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
			UID:        "test-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "secret-vol",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestKubernetesCollector_ConfigMapChange(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true

	cm := newTestConfigMap("my-config", "default", map[string]string{"key1": "value1"})
	client := fake.NewSimpleClientset(cm)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: baseline init — no event expected
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		t.Fatalf("unexpected event on first poll: %s", event.Summary)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event on first-seen
	}

	// Change ConfigMap data
	cm.Data["key1"] = "value2"
	cm.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Second poll: data changed — event expected
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		if event.ChangeType != "configmap_update" {
			t.Errorf("expected change_type configmap_update, got %s", event.ChangeType)
		}
		if event.Metadata["namespace"] != "default" {
			t.Errorf("expected namespace default, got %s", event.Metadata["namespace"])
		}
		if event.Metadata["kind"] != "ConfigMap" {
			t.Errorf("expected kind ConfigMap, got %s", event.Metadata["kind"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for configmap change event")
	}
}

func TestKubernetesCollector_ConfigMapDeduplication(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true

	cm := newTestConfigMap("dedup-config", "default", map[string]string{"key": "val"})
	client := fake.NewSimpleClientset(cm)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on init")
	case <-time.After(100 * time.Millisecond):
	}

	// Second poll: unchanged data — no event
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on unchanged data")
	case <-time.After(100 * time.Millisecond):
	}

	// Change data
	cm.Data["key"] = "new-val"
	cm.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Third poll: changed — event expected
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for change event")
	}

	// Fourth poll: unchanged again — no event
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event after change was already detected")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestKubernetesCollector_SecretChange(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchSecrets = true

	secret := newTestSecret("my-secret", "default", map[string][]byte{"password": []byte("hunter2")})
	client := fake.NewSimpleClientset(secret)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init — no event
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on init")
	case <-time.After(100 * time.Millisecond):
	}

	// Change secret data
	secret.Data["password"] = []byte("new-password")
	secret.ResourceVersion = "2"
	if _, err := client.CoreV1().Secrets("default").Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Second poll: changed — event expected
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		if event.ChangeType != "secret_update" {
			t.Errorf("expected change_type secret_update, got %s", event.ChangeType)
		}
		// Verify data_hash is NOT in metadata (security)
		if _, hasHash := event.Metadata["data_hash"]; hasHash {
			t.Error("secret event metadata should NOT contain data_hash")
		}
		if event.Metadata["kind"] != "Secret" {
			t.Errorf("expected kind Secret, got %s", event.Metadata["kind"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for secret change event")
	}
}

func TestKubernetesCollector_SecretSkipsServiceAccountTokens(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchSecrets = true

	saSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-token",
			Namespace: "default",
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{"token": []byte("abc")},
	}
	client := fake.NewSimpleClientset(saSecret)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init (SA token should be skipped entirely)
	coll.CollectAndSend(ctx, eventChan)

	// Change the SA token data
	saSecret.Data["token"] = []byte("xyz")
	saSecret.ResourceVersion = "2"
	if _, err := client.CoreV1().Secrets("default").Update(ctx, saSecret, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Second poll: should still not produce event since SA tokens are skipped
	coll.CollectAndSend(ctx, eventChan)
	select {
	case event := <-eventChan:
		t.Fatalf("unexpected event for ServiceAccountToken secret: %s", event.Summary)
	case <-time.After(100 * time.Millisecond):
		// Expected: SA token secrets are ignored
	}
}

func TestKubernetesCollector_AffectedServicesResolution(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.WatchSecrets = true

	cm := newTestConfigMap("shared-config", "default", map[string]string{"key": "val"})
	secret := newTestSecret("shared-secret", "default", map[string][]byte{"pass": []byte("123")})
	depWithCM := newTestDeploymentWithConfigMapRef("app-with-cm", "default", "shared-config")
	depWithSecret := newTestDeploymentWithSecretRef("app-with-secret", "default", "shared-secret")
	depUnrelated := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-unrelated",
			Namespace:  "default",
			Generation: 1,
			UID:        "test-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "app:latest"}},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(cm, secret, depWithCM, depWithSecret, depUnrelated)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on init")
	case <-time.After(100 * time.Millisecond):
	}

	// Change ConfigMap
	cm.Data["key"] = "new-val"
	cm.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Change Secret
	secret.Data["pass"] = []byte("456")
	secret.ResourceVersion = "2"
	if _, err := client.CoreV1().Secrets("default").Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	coll.CollectAndSend(ctx, eventChan)

	events := make([]domain.ChangeEvent, 0)
	for {
		select {
		case e := <-eventChan:
			events = append(events, e)
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}
done:

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	for _, event := range events {
		if event.ChangeType == "configmap_update" {
			if len(event.AffectedServices) != 1 || event.AffectedServices[0] != "app-with-cm" {
				t.Errorf("configmap event: expected AffectedServices=[app-with-cm], got %v", event.AffectedServices)
			}
		} else if event.ChangeType == "secret_update" {
			if len(event.AffectedServices) != 1 || event.AffectedServices[0] != "app-with-secret" {
				t.Errorf("secret event: expected AffectedServices=[app-with-secret], got %v", event.AffectedServices)
			}
		}
	}
}

func TestKubernetesCollector_ConfigMapNamespaceFilter(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.Namespaces = []string{"watched-ns"}

	cmWatched := newTestConfigMap("cm-watched", "watched-ns", map[string]string{"k": "v"})
	cmUnwatched := newTestConfigMap("cm-unwatched", "other-ns", map[string]string{"k": "v"})

	client := fake.NewSimpleClientset(cmWatched, cmUnwatched)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on init")
	case <-time.After(100 * time.Millisecond):
	}

	// Change both ConfigMaps
	cmWatched.Data["k"] = "v2"
	cmWatched.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("watched-ns").Update(ctx, cmWatched, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	cmUnwatched.Data["k"] = "v2"
	cmUnwatched.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("other-ns").Update(ctx, cmUnwatched, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	coll.CollectAndSend(ctx, eventChan)

	var events []domain.ChangeEvent
	for {
		select {
		case e := <-eventChan:
			events = append(events, e)
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}
done:

	if len(events) != 1 {
		t.Fatalf("expected 1 event (watched ns only), got %d", len(events))
	}
	if events[0].Metadata["namespace"] != "watched-ns" {
		t.Errorf("expected event from watched-ns, got %s", events[0].Metadata["namespace"])
	}
}

func TestKubernetesCollector_ArgoCDAutoDetection(t *testing.T) {
	cfg := config.Config{}
	// This test now verifies that auto-detection is *disabled* and we must rely on explicit annotations.
	// For these tests, we will not provide a `valiant.io/source` annotation.
	cfg.Kubernetes.AllowedSources = []string{"argocd"}

	t.Run("no auto-detect via tracking-id annotation", func(t *testing.T) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-app",
				Namespace: "default",
				Annotations: map[string]string{
					"argocd.argoproj.io/tracking-id": "my-app:apps/Deployment:default/argocd-app",
				},
				Generation: 1,
				UID:        "test-uid",
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
				},
			},
		}

		client := fake.NewSimpleClientset(dep)
		coll, err := collector.NewKubernetesCollector(cfg, client)
		if err != nil {
			t.Fatal(err)
		}
		eventChan := make(chan domain.ChangeEvent, 10)
		ctx := context.Background()

		coll.CollectAndSend(ctx, eventChan)

		select {
		case event := <-eventChan:
			if event.TriggerType != "kubernetes-api" {
				t.Errorf("expected fallback TriggerType 'kubernetes-api', got %q", event.TriggerType)
			}
			if name, ok := event.Metadata["argocd_app_name"]; ok && name != "" {
				t.Errorf("expected argocd_app_name to be empty, got %q", name)
			}
			if intentSource, ok := event.Metadata["intent_source"]; ok && intentSource != "" {
				t.Errorf("expected empty intent_source, got %q", intentSource)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("no auto-detect via managed-by annotation", func(t *testing.T) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managed-app",
				Namespace: "default",
				Annotations: map[string]string{
					"argocd.argoproj.io/managed-by": "argocd-instance",
				},
				Generation: 1,
				UID:        "test-uid-2",
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
				},
			},
		}

		client := fake.NewSimpleClientset(dep)
		coll, err := collector.NewKubernetesCollector(cfg, client)
		if err != nil {
			t.Fatal(err)
		}
		eventChan := make(chan domain.ChangeEvent, 10)
		ctx := context.Background()

		coll.CollectAndSend(ctx, eventChan)

		select {
		case event := <-eventChan:
			if event.TriggerType != "kubernetes-api" {
				t.Errorf("expected fallback TriggerType 'kubernetes-api', got %q", event.TriggerType)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("no auto-detect via instance label", func(t *testing.T) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-app",
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "my-argo-app",
				},
				Generation: 1,
				UID:        "test-uid-3",
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
				},
			},
		}

		client := fake.NewSimpleClientset(dep)
		coll, err := collector.NewKubernetesCollector(cfg, client)
		if err != nil {
			t.Fatal(err)
		}
		eventChan := make(chan domain.ChangeEvent, 10)
		ctx := context.Background()

		coll.CollectAndSend(ctx, eventChan)

		select {
		case event := <-eventChan:
			if event.TriggerType != "kubernetes-api" {
				t.Errorf("expected fallback TriggerType 'kubernetes-api', got %q", event.TriggerType)
			}
			if name, ok := event.Metadata["argocd_app_name"]; ok && name != "" {
				t.Errorf("expected argocd_app_name to be empty, got %q", name)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("explicit annotation takes priority over ArgoCD labels", func(t *testing.T) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "priority-app",
				Namespace: "default",
				Annotations: map[string]string{
					"valiant.io/source":              "helm",
					"argocd.argoproj.io/tracking-id": "my-app:apps/Deployment:default/priority-app",
				},
				Labels: map[string]string{
					"app.kubernetes.io/instance": "my-argo-app",
				},
				Generation: 1,
				UID:        "test-uid-4",
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
				},
			},
		}

		cfgWithHelm := config.Config{}
		cfgWithHelm.Kubernetes.AllowedSources = []string{"argocd", "helm"}

		client := fake.NewSimpleClientset(dep)
		coll, err := collector.NewKubernetesCollector(cfgWithHelm, client)
		if err != nil {
			t.Fatal(err)
		}
		eventChan := make(chan domain.ChangeEvent, 10)
		ctx := context.Background()

		coll.CollectAndSend(ctx, eventChan)

		select {
		case event := <-eventChan:
			if event.TriggerType != "helm" {
				t.Errorf("expected TriggerType 'helm' (explicit annotation priority), got %q", event.TriggerType)
			}
			if event.Metadata["intent_source"] != "helm" {
				t.Errorf("expected intent_source 'helm', got %q", event.Metadata["intent_source"])
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("fallback to kubernetes-api when no indicators", func(t *testing.T) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "plain-app",
				Namespace:  "default",
				Generation: 1,
				UID:        "test-uid-5",
			},
			Status: appsv1.DeploymentStatus{
				Conditions: []appsv1.DeploymentCondition{
					{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
					{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
				},
			},
		}

		client := fake.NewSimpleClientset(dep)
		coll, err := collector.NewKubernetesCollector(cfg, client)
		if err != nil {
			t.Fatal(err)
		}
		eventChan := make(chan domain.ChangeEvent, 10)
		ctx := context.Background()

		coll.CollectAndSend(ctx, eventChan)

		select {
		case event := <-eventChan:
			if event.TriggerType != "kubernetes-api" {
				t.Errorf("expected TriggerType 'kubernetes-api', got %q", event.TriggerType)
			}
			if event.Metadata["argocd_app_name"] != "" {
				t.Errorf("expected empty argocd_app_name, got %q", event.Metadata["argocd_app_name"])
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	})
}

func TestKubernetesCollector_ConfigMapAnnotationFilter(t *testing.T) {
	cfg := config.Config{}
	cfg.Kubernetes.WatchConfigMaps = true
	cfg.Kubernetes.RequireAnnotation = true
	cfg.Kubernetes.AllowedSources = []string{"argocd"}

	// Annotated ConfigMap
	cmAnnotated := newTestConfigMap("cm-annotated", "default", map[string]string{"k": "v"})

	// Unannotated ConfigMap
	cmUnannotated := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-unannotated",
			Namespace: "default",
		},
		Data: map[string]string{"k": "v"},
	}

	client := fake.NewSimpleClientset(cmAnnotated, cmUnannotated)

	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	// First poll: init
	coll.CollectAndSend(ctx, eventChan)
	select {
	case <-eventChan:
		t.Fatal("unexpected event on init")
	case <-time.After(100 * time.Millisecond):
	}

	// Change both ConfigMaps
	cmAnnotated.Data["k"] = "v2"
	cmAnnotated.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("default").Update(ctx, cmAnnotated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	cmUnannotated.Data["k"] = "v2"
	cmUnannotated.ResourceVersion = "2"
	if _, err := client.CoreV1().ConfigMaps("default").Update(ctx, cmUnannotated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	coll.CollectAndSend(ctx, eventChan)

	var events []domain.ChangeEvent
	for {
		select {
		case e := <-eventChan:
			events = append(events, e)
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}
done:

	if len(events) != 1 {
		t.Fatalf("expected 1 event (annotated only), got %d", len(events))
	}
	if events[0].Summary != "ConfigMap default/cm-annotated data changed" {
		t.Errorf("unexpected event summary: %s", events[0].Summary)
	}
}

func TestSelectAppContainer(t *testing.T) {
	t.Run("single container", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "app", Image: "registry/app:v1"},
		}
		image, name := collector.SelectAppContainer(containers, nil)
		if image != "registry/app:v1" || name != "app" {
			t.Errorf("expected registry/app:v1 / app, got %s / %s", image, name)
		}
	})

	t.Run("skip istio-proxy sidecar", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "istio-proxy", Image: "istio/proxyv2:1.20"},
			{Name: "api", Image: "registry/api:v2"},
		}
		image, name := collector.SelectAppContainer(containers, nil)
		if image != "registry/api:v2" || name != "api" {
			t.Errorf("expected registry/api:v2 / api, got %s / %s", image, name)
		}
	})

	t.Run("skip multiple sidecars", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "istio-proxy", Image: "istio/proxyv2:1.20"},
			{Name: "vault-agent", Image: "vault:1.15"},
			{Name: "worker", Image: "registry/worker:v3"},
		}
		image, name := collector.SelectAppContainer(containers, nil)
		if image != "registry/worker:v3" || name != "worker" {
			t.Errorf("expected registry/worker:v3 / worker, got %s / %s", image, name)
		}
	})

	t.Run("annotation override", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "sidecar", Image: "sidecar:v1"},
			{Name: "main-app", Image: "registry/main:v4"},
		}
		annotations := map[string]string{"valiant.io/container": "main-app"}
		image, name := collector.SelectAppContainer(containers, annotations)
		if image != "registry/main:v4" || name != "main-app" {
			t.Errorf("expected registry/main:v4 / main-app, got %s / %s", image, name)
		}
	})

	t.Run("annotation for missing container falls back to non-sidecar", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "istio-proxy", Image: "istio/proxyv2:1.20"},
			{Name: "api", Image: "registry/api:v5"},
		}
		annotations := map[string]string{"valiant.io/container": "nonexistent"}
		image, name := collector.SelectAppContainer(containers, annotations)
		if image != "registry/api:v5" || name != "api" {
			t.Errorf("expected registry/api:v5 / api, got %s / %s", image, name)
		}
	})

	t.Run("all sidecars falls back to first", func(t *testing.T) {
		containers := []corev1.Container{
			{Name: "istio-proxy", Image: "istio/proxyv2:1.20"},
			{Name: "envoy", Image: "envoy:1.28"},
		}
		image, name := collector.SelectAppContainer(containers, nil)
		if image != "istio/proxyv2:1.20" || name != "istio-proxy" {
			t.Errorf("expected istio/proxyv2:1.20 / istio-proxy, got %s / %s", image, name)
		}
	})

	t.Run("empty containers", func(t *testing.T) {
		image, name := collector.SelectAppContainer(nil, nil)
		if image != "" || name != "" {
			t.Errorf("expected empty, got %s / %s", image, name)
		}
	})
}

func TestKubernetesCollector_DeploymentMetadataKeys(t *testing.T) {
	cfg := config.Config{}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "meta-app",
			Namespace:  "payment-ns",
			Generation: 1,
			UID:        "test-uid-meta",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "registry/payment-service:abc123"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{Type: "Available", Status: "True", LastUpdateTime: metav1.Now()},
				{Type: "Progressing", Reason: "NewReplicaSetAvailable"},
			},
		},
	}

	client := fake.NewSimpleClientset(dep)
	coll, err := collector.NewKubernetesCollector(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	eventChan := make(chan domain.ChangeEvent, 10)
	ctx := context.Background()

	coll.CollectAndSend(ctx, eventChan)

	select {
	case event := <-eventChan:
		// Verify "env" metadata matches namespace
		if event.Metadata["env"] != "payment-ns" {
			t.Errorf("expected metadata env='payment-ns', got %q", event.Metadata["env"])
		}
		// Verify "namespace" is still present (backward compat)
		if event.Metadata["namespace"] != "payment-ns" {
			t.Errorf("expected metadata namespace='payment-ns', got %q", event.Metadata["namespace"])
		}
		// Verify "image_tag" matches container image
		if event.Metadata["image_tag"] != "registry/payment-service:abc123" {
			t.Errorf("expected metadata image_tag='registry/payment-service:abc123', got %q", event.Metadata["image_tag"])
		}
		// Verify "image" is also present
		if event.Metadata["image"] != "registry/payment-service:abc123" {
			t.Errorf("expected metadata image='registry/payment-service:abc123', got %q", event.Metadata["image"])
		}
		// Verify Source field
		if event.Source != "kubernetes" {
			t.Errorf("expected Source='kubernetes', got %q", event.Source)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}
