/*
Copyright 2025.

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

package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var _ = Describe("FreeboxMachine Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		freeboxmachine := &infrastructurev1alpha1.FreeboxMachine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind FreeboxMachine")
			err := k8sClient.Get(ctx, typeNamespacedName, freeboxmachine)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.FreeboxMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: infrastructurev1alpha1.FreeboxMachineSpec{
						Name:          "test-vm",
						VCPUs:         2,
						MemoryMB:      2048,
						DiskSizeBytes: 20 * 1024 * 1024 * 1024, // 20GB
						ImageURL:      "",                      // Empty URL to skip download logic in tests
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1alpha1.FreeboxMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance FreeboxMachine")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FreeboxMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When reconciling with paused annotation", func() {
		const resourceName = "test-paused"
		const clusterName = "test-cluster"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Cluster to own the FreeboxMachine")
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(true),
				},
			}
			err := k8sClient.Create(ctx, cluster)
			if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind FreeboxMachine with paused annotation")
			freeboxmachine := &infrastructurev1alpha1.FreeboxMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: clusterName,
					},
					Annotations: map[string]string{
						"cluster.x-k8s.io/paused": "true",
					},
				},
				Spec: infrastructurev1alpha1.FreeboxMachineSpec{
					Name:          "test-vm",
					VCPUs:         2,
					MemoryMB:      2048,
					DiskSizeBytes: 20 * 1024 * 1024 * 1024,
					ImageURL:      "",
				},
			}
			err = k8sClient.Create(ctx, freeboxmachine)
			if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			By("Cleaning up FreeboxMachine")
			resource := &infrastructurev1alpha1.FreeboxMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleaning up Cluster")
			cluster := &clusterv1.Cluster{}
			clusterKey := types.NamespacedName{Name: clusterName, Namespace: "default"}
			err = k8sClient.Get(ctx, clusterKey, cluster)
			if err == nil {
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}
		})

		It("should not reconcile when paused annotation is set", func() {
			By("Reconciling the paused FreeboxMachine")
			controllerReconciler := &FreeboxMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the FreeboxMachine was not reconciled (should return early)")
			Expect(result.RequeueAfter).To(BeZero(), "Should not requeue when paused")

			// Verify the FreeboxMachine was NOT modified (no phase, no taskID, etc.)
			// This confirms reconciliation was skipped entirely
			freeboxMachine := &infrastructurev1alpha1.FreeboxMachine{}
			err = k8sClient.Get(ctx, typeNamespacedName, freeboxMachine)
			Expect(err).NotTo(HaveOccurred())
			Expect(freeboxMachine.Status.Phase).To(BeEmpty(), "Phase should be empty when paused")
		})
	})
})

func TestStripCompressionSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"nocloud.raw.xz", "nocloud.raw"},
		{"image.img.gz", "image.img"},
		{"archive.tar.bz2", "archive.tar"},
		{"file.zip", "file"},
		{"nocloud.tar", "nocloud"},
		{"plain.raw", "plain"}, // no compression extension — falls back to path.Ext trimming
		{"noext", "noext"},
		{"UPPER.XZ", "UPPER"},
		{"mixed.Gz", "mixed"},
	}
	for _, tc := range tests {
		got := stripCompressionSuffix(tc.input)
		if got != tc.want {
			t.Errorf("stripCompressionSuffix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
