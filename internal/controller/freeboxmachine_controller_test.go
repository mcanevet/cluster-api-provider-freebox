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

var _ = Describe("FreeboxMachine Controller", func() {
	Context("When reconciling a FreeboxMachine", func() {
		const resourceName = "test-freebox-machine"
		const resourceNamespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		Context("When FreeboxMachine does not exist", func() {
			It("should handle not found gracefully", func() {
				By("setting up the controller reconciler")
				controllerReconciler := &FreeboxMachineReconciler{
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

		Context("When FreeboxMachine exists", func() {
			var freeboxMachine *infrastructurev1alpha1.FreeboxMachine
			var controllerReconciler *FreeboxMachineReconciler

			BeforeEach(func() {
				By("setting up the controller reconciler")
				controllerReconciler = &FreeboxMachineReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				By("creating a FreeboxMachine resource")
				freeboxMachine = &infrastructurev1alpha1.FreeboxMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: infrastructurev1alpha1.FreeboxMachineSpec{
						CPUs:     2,
						Memory:   4,
						DiskSize: 20,
						ImageURL: "https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img",
					},
				}
				Expect(k8sClient.Create(ctx, freeboxMachine)).To(Succeed())
			})

			AfterEach(func() {
				By("cleaning up the FreeboxMachine resource")
				resource := &infrastructurev1alpha1.FreeboxMachine{}
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				if err == nil {
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				}
			})

			It("should set initialization.provisioned to true", func() {
				By("reconciling the FreeboxMachine")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying initialization.provisioned is set to true")
				updatedMachine := &infrastructurev1alpha1.FreeboxMachine{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedMachine)).To(Succeed())

				Expect(updatedMachine.Status.Initialization).NotTo(BeNil())
				Expect(updatedMachine.Status.Initialization.Provisioned).NotTo(BeNil())
				Expect(*updatedMachine.Status.Initialization.Provisioned).To(BeTrue())
			})

			It("should set a provider ID", func() {
				By("reconciling the FreeboxMachine")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying provider ID is set")
				updatedMachine := &infrastructurev1alpha1.FreeboxMachine{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedMachine)).To(Succeed())

				Expect(updatedMachine.Spec.ProviderID).NotTo(BeEmpty())
				Expect(updatedMachine.Spec.ProviderID).To(HavePrefix("freebox:///"))
			})

			It("should be idempotent - not change status if already provisioned", func() {
				By("setting initial provisioned status and provider ID")
				provisioned := true
				freeboxMachine.Status.Initialization = &infrastructurev1alpha1.FreeboxMachineInitializationStatus{
					Provisioned: &provisioned,
				}
				freeboxMachine.Spec.ProviderID = "freebox:///vm-12345"
				Expect(k8sClient.Update(ctx, freeboxMachine)).To(Succeed())
				Expect(k8sClient.Status().Update(ctx, freeboxMachine)).To(Succeed())

				By("reconciling the FreeboxMachine again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying status and provider ID remain unchanged")
				updatedMachine := &infrastructurev1alpha1.FreeboxMachine{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedMachine)).To(Succeed())

				Expect(updatedMachine.Status.Initialization).NotTo(BeNil())
				Expect(updatedMachine.Status.Initialization.Provisioned).NotTo(BeNil())
				Expect(*updatedMachine.Status.Initialization.Provisioned).To(BeTrue())
				Expect(updatedMachine.Spec.ProviderID).To(Equal("freebox:///vm-12345"))
			})

			It("should preserve VM configuration from spec", func() {
				By("reconciling the FreeboxMachine")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("verifying VM configuration is preserved")
				updatedMachine := &infrastructurev1alpha1.FreeboxMachine{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, updatedMachine)).To(Succeed())

				Expect(updatedMachine.Spec.CPUs).To(Equal(int64(2)))
				Expect(updatedMachine.Spec.Memory).To(Equal(int64(4)))
				Expect(updatedMachine.Spec.DiskSize).To(Equal(int64(20)))
				Expect(updatedMachine.Spec.ImageURL).To(ContainSubstring("ubuntu-22.04"))
			})
		})
	})
})
