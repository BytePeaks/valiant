package shared

import (
	"time"
	"valiant/internal/domain"
	"valiant/tests/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// SampleChangeEvent creates a test change event
func SampleChangeEvent() domain.ChangeEvent {
	return common.SampleChangeEvent()
}

// SampleCIEvent creates a test CI (Intent) event
func SampleCIEvent() domain.ChangeEvent {
	return common.SampleCIEvent()
}

// SampleMetricValues creates test metric values
func SampleMetricValues(errorRate, latencyP95, rps, cpu, memory float64) domain.MetricValues {
	return common.SampleMetricValues(errorRate, latencyP95, rps, cpu, memory)
}

// NoImpactMetrics creates baseline metrics with no change
func NoImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.NoImpactMetrics()
}

// LowImpactMetrics creates metrics with small error rate increase
func LowImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.LowImpactMetrics()
}

// MediumImpactMetrics creates metrics with moderate degradation
func MediumImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.MediumImpactMetrics()
}

// HighImpactMetrics creates metrics with severe degradation
func HighImpactMetrics() (baseline, impact domain.MetricValues) {
	return common.HighImpactMetrics()
}

// SampleDeployment creates a test Kubernetes Deployment object with minimal status
func SampleDeployment(name, namespace, image, sha string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
			Annotations: map[string]string{
				"valiant.io/source": "cicd",
				"git_commit_sha":    sha,
			},
			Generation: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: image,
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
		},
	}
}

// SetDeploymentReady sets the status of a deployment to ready, including conditions.
func SetDeploymentReady(d *appsv1.Deployment) *appsv1.Deployment {
	d.Status.AvailableReplicas = *d.Spec.Replicas
	d.Status.ReadyReplicas = *d.Spec.Replicas
	d.Status.UpdatedReplicas = *d.Spec.Replicas
	d.Status.ObservedGeneration = d.Generation
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentProgressing,
			Status: corev1.ConditionTrue,
			Reason: "NewReplicaSetAvailable",
		},
		{
			Type:           appsv1.DeploymentAvailable,
			Status:         corev1.ConditionTrue,
			LastUpdateTime: metav1.NewTime(time.Now()), // Recent update time
		},
	}
	return d
}

// SetDeploymentNotReady sets the status of a deployment to not ready
func SetDeploymentNotReady(d *appsv1.Deployment) *appsv1.Deployment {
	d.Status.AvailableReplicas = 0
	d.Status.ReadyReplicas = 0
	d.Status.UpdatedReplicas = 0
	return d
}

// int32Ptr returns a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
}

// NewFakeKubeClient creates a fake Kubernetes clientset with initial objects.
func NewFakeKubeClient(objects ...runtime.Object) *fake.Clientset {
	return fake.NewSimpleClientset(objects...)
}

// SampleConfigMap creates a test Kubernetes ConfigMap object
func SampleConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"valiant.io/source": "cicd", // For collector filtering
			},
		},
		Data: data,
	}
}

// SampleSecret creates a test Kubernetes Secret object
func SampleSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"valiant.io/source": "cicd", // For collector filtering
			},
		},
		Data: data,
		Type: corev1.SecretTypeOpaque, // Standard secret type
	}
}
