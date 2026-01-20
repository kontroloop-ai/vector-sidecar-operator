/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/amitde789696/vector-sidecar-operator/api/v1alpha1"
)

var _ = Describe("VectorSidecar Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a VectorSidecar", func() {
		ctx := context.Background()

		It("Should successfully inject sidecar into matching deployment", func() {
			// Create ConfigMap for Vector config
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vector-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"vector.yaml": "# Vector configuration\nsources: {}\nsinks: {}",
				},
			}

			// Create a deployment with matching labels
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"observability": "vector",
						"app":           "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}

			// Create VectorSidecar CR
			vectorSidecar := &observabilityv1alpha1.VectorSidecar{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vectorsidecar",
					Namespace: "default",
				},
				Spec: observabilityv1alpha1.VectorSidecarSpec{
					Enabled: true,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"observability": "vector",
						},
					},
					Sidecar: observabilityv1alpha1.SidecarConfig{
						Name:  "vector",
						Image: "timberio/vector:0.35.0",
						Config: observabilityv1alpha1.VectorConfig{
							ConfigMapRef: &observabilityv1alpha1.ConfigMapRef{
								Name: "vector-config",
								Key:  "vector.yaml",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "varlog",
								MountPath: "/var/log",
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "varlog",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/log",
								},
							},
						},
					},
				},
			}

			// Setup fake client
			s := scheme.Scheme
			_ = observabilityv1alpha1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(configMap, deployment, vectorSidecar).
				Build()

			recorder := record.NewFakeRecorder(10)

			reconciler := &VectorSidecarReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: recorder,
			}

			// Test reconciliation
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-vectorsidecar",
					Namespace: "default",
				},
			}

			// First reconcile to add finalizer
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			// Second reconcile to inject sidecar
			result, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Verify the deployment was updated with sidecar
			updatedDeployment := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-deployment",
				Namespace: "default",
			}, updatedDeployment)
			Expect(err).NotTo(HaveOccurred())

			// Check that Vector container was added
			containers := updatedDeployment.Spec.Template.Spec.Containers
			Expect(len(containers)).To(Equal(2))

			vectorFound := false
			for _, container := range containers {
				if container.Name == "vector" {
					vectorFound = true
					Expect(container.Image).To(Equal("timberio/vector:0.35.0"))
					Expect(len(container.VolumeMounts)).To(BeNumerically(">", 0))
					break
				}
			}
			Expect(vectorFound).To(BeTrue())

			// Check annotations were added
			Expect(updatedDeployment.Annotations[AnnotationInjected]).To(Equal("true"))
			Expect(updatedDeployment.Annotations[AnnotationVectorSidecarName]).To(Equal("test-vectorsidecar"))

			// Check volumes were added
			Expect(len(updatedDeployment.Spec.Template.Spec.Volumes)).To(BeNumerically(">", 0))
		})

		It("Should handle ConfigMap validation failure", func() {
			// Create VectorSidecar CR with non-existent ConfigMap
			vectorSidecar := &observabilityv1alpha1.VectorSidecar{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vectorsidecar-invalid",
					Namespace: "default",
				},
				Spec: observabilityv1alpha1.VectorSidecarSpec{
					Enabled: true,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"observability": "vector",
						},
					},
					Sidecar: observabilityv1alpha1.SidecarConfig{
						Name:  "vector",
						Image: "timberio/vector:0.35.0",
						Config: observabilityv1alpha1.VectorConfig{
							ConfigMapRef: &observabilityv1alpha1.ConfigMapRef{
								Name: "non-existent-config",
								Key:  "vector.yaml",
							},
						},
					},
				},
			}

			s := scheme.Scheme
			_ = observabilityv1alpha1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(vectorSidecar).
				Build()

			recorder := record.NewFakeRecorder(10)

			reconciler := &VectorSidecarReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: recorder,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-vectorsidecar-invalid",
					Namespace: "default",
				},
			}

			// First reconcile to add finalizer
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile should handle validation error
			result, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Minute))

			// Check that ConfigValid condition is false
			updatedVS := &observabilityv1alpha1.VectorSidecar{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-vectorsidecar-invalid",
				Namespace: "default",
			}, updatedVS)
			Expect(err).NotTo(HaveOccurred())

			configValidCondition := findCondition(updatedVS.Status.Conditions, observabilityv1alpha1.ConditionTypeConfigValid)
			Expect(configValidCondition).NotTo(BeNil())
			Expect(configValidCondition.Status).To(Equal(metav1.ConditionFalse))
		})

		It("Should remove sidecar when enabled is set to false", func() {
			// Create ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vector-config-2",
					Namespace: "default",
				},
				Data: map[string]string{
					"vector.yaml": "sources: {}\nsinks: {}",
				},
			}

			// Create deployment with Vector sidecar already injected
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-2",
					Namespace: "default",
					Labels: map[string]string{
						"observability": "vector",
					},
					Annotations: map[string]string{
						AnnotationInjected:          "true",
						AnnotationVectorSidecarName: "test-vectorsidecar-disable",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx:latest",
								},
								{
									Name:  "vector",
									Image: "timberio/vector:0.35.0",
								},
							},
						},
					},
				},
			}

			// Create VectorSidecar CR with enabled=false
			vectorSidecar := &observabilityv1alpha1.VectorSidecar{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vectorsidecar-disable",
					Namespace: "default",
				},
				Spec: observabilityv1alpha1.VectorSidecarSpec{
					Enabled: false,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"observability": "vector",
						},
					},
					Sidecar: observabilityv1alpha1.SidecarConfig{
						Image: "timberio/vector:0.35.0",
						Config: observabilityv1alpha1.VectorConfig{
							ConfigMapRef: &observabilityv1alpha1.ConfigMapRef{
								Name: "vector-config-2",
							},
						},
					},
				},
			}

			s := scheme.Scheme
			_ = observabilityv1alpha1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(configMap, deployment, vectorSidecar).
				Build()

			recorder := record.NewFakeRecorder(10)

			reconciler := &VectorSidecarReconciler{
				Client:   fakeClient,
				Scheme:   s,
				Recorder: recorder,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-vectorsidecar-disable",
					Namespace: "default",
				},
			}

			// First reconcile to add finalizer
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile should remove sidecar
			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Verify Vector container was removed
			updatedDeployment := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-deployment-2",
				Namespace: "default",
			}, updatedDeployment)
			Expect(err).NotTo(HaveOccurred())

			// Check that only app container remains
			containers := updatedDeployment.Spec.Template.Spec.Containers
			Expect(len(containers)).To(Equal(1))
			Expect(containers[0].Name).To(Equal("app"))
		})

		It("Should calculate consistent injection hash", func() {
			vectorSidecar := &observabilityv1alpha1.VectorSidecar{
				Spec: observabilityv1alpha1.VectorSidecarSpec{
					Sidecar: observabilityv1alpha1.SidecarConfig{
						Image: "timberio/vector:0.35.0",
						Config: observabilityv1alpha1.VectorConfig{
							ConfigMapRef: &observabilityv1alpha1.ConfigMapRef{
								Name: "vector-config",
							},
						},
					},
				},
			}

			s := scheme.Scheme
			reconciler := &VectorSidecarReconciler{
				Scheme: s,
			}

			hash1, err := reconciler.calculateInjectionHash(vectorSidecar)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash1).NotTo(BeEmpty())

			// Calculate again to ensure consistency
			hash2, err := reconciler.calculateInjectionHash(vectorSidecar)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash2).To(Equal(hash1))

			// Change image and verify hash changes
			vectorSidecar.Spec.Sidecar.Image = "timberio/vector:0.36.0"
			hash3, err := reconciler.calculateInjectionHash(vectorSidecar)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash3).NotTo(Equal(hash1))
		})
	})
})

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
