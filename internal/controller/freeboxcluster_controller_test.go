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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

var _ = Describe("FreeboxCluster Controller", func() {
	Context("When reconciling a FreeboxCluster", func() {
		const resourceName = "test-freebox-cluster"
		const resourceNamespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		Context("When FreeboxCluster does not exist", func() {
			It("should handle not found gracefully", func() {
				By("setting up the controller reconciler")
				controllerReconciler := &FreeboxClusterReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Try to reconcile a non-existent resource
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "non-existent",
						Namespace: resourceNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("When FreeboxCluster exists", func() {
			var freeboxCluster *infrastructurev1alpha1.FreeboxCluster
			var controllerReconciler *FreeboxClusterReconciler

			BeforeEach(func() {
				By("setting up the controller reconciler")
				controllerReconciler = &FreeboxClusterReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				By("creating a FreeboxCluster resource")
				freeboxCluster = &infrastructurev1alpha1.FreeboxCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: infrastructurev1alpha1.FreeboxClusterSpec{
						ControlPlaneEndpoint: infrastructurev1alpha1.APIEndpoint{
							Host: "192.168.1.100",
							Port: 6443,
						},
					},
				}
				Expect(k8sClient.Create(ctx, freeboxCluster)).To(Succeed())
			})

			AfterEach(func() {
				By("cleaning up the FreeboxCluster resource")
				resource := &infrastructurev1alpha1.FreeboxCluster{}
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				if err == nil {
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				}
			})

			It("should set initialization.provisioned to true", func() {
				By("reconciling the FreeboxCluster")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying initialization.provisioned is set to true")
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())

				Expect(updatedCluster.Status.Initialization).NotTo(BeNil())
				Expect(updatedCluster.Status.Initialization.Provisioned).NotTo(BeNil())
				Expect(*updatedCluster.Status.Initialization.Provisioned).To(BeTrue())
			})

			It("should be idempotent - not change status if already provisioned", func() {
				By("setting initial provisioned status")
				provisioned := true
				freeboxCluster.Status.Initialization = &infrastructurev1alpha1.FreeboxClusterInitializationStatus{
					Provisioned: &provisioned,
				}
				Expect(k8sClient.Status().Update(ctx, freeboxCluster)).To(Succeed())

				By("reconciling the FreeboxCluster again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying status remains unchanged")
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())

				Expect(updatedCluster.Status.Initialization).NotTo(BeNil())
				Expect(updatedCluster.Status.Initialization.Provisioned).NotTo(BeNil())
				Expect(*updatedCluster.Status.Initialization.Provisioned).To(BeTrue())
			})

			It("should maintain control plane endpoint from spec", func() {
				By("reconciling the FreeboxCluster")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying control plane endpoint is preserved")
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())

				Expect(updatedCluster.Spec.ControlPlaneEndpoint.Host).To(Equal("192.168.1.100"))
				Expect(updatedCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(6443)))
			})
		})
	})
})
