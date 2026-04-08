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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

var _ = Describe("FreeboxCluster Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		freeboxcluster := &infrastructurev1alpha1.FreeboxCluster{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind FreeboxCluster")
			err := k8sClient.Get(ctx, typeNamespacedName, freeboxcluster)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.FreeboxCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: infrastructurev1alpha1.FreeboxClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{
							Host: "192.168.1.100",
							Port: 6443,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1alpha1.FreeboxCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance FreeboxCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FreeboxClusterReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When reconciling with paused Cluster", func() {
		const resourceName = "test-paused-cluster"
		const clusterName = "test-cluster"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		var cluster *clusterv1.Cluster

		BeforeEach(func() {
			By("creating a Cluster with Spec.Paused=true")
			cluster = &clusterv1.Cluster{
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

			// Refresh to get the UID
			err = k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, cluster)
			Expect(err).NotTo(HaveOccurred())

			By("creating the FreeboxCluster owned by the paused Cluster")
			freeboxCluster := &infrastructurev1alpha1.FreeboxCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "cluster.x-k8s.io/v1beta2",
							Kind:               "Cluster",
							Name:               clusterName,
							UID:                cluster.UID,
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
				},
				Spec: infrastructurev1alpha1.FreeboxClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
				},
			}
			err = k8sClient.Create(ctx, freeboxCluster)
			if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			By("Cleaning up FreeboxCluster")
			resource := &infrastructurev1alpha1.FreeboxCluster{}
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

		It("should not reconcile when the owner Cluster is paused", func() {
			By("Reconciling the FreeboxCluster with paused owner")
			controllerReconciler := &FreeboxClusterReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the FreeboxCluster was not reconciled")
			Expect(result.RequeueAfter).To(BeZero(), "Should not requeue when paused")

			// Verify the FreeboxCluster status was NOT modified
			freeboxCluster := &infrastructurev1alpha1.FreeboxCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, freeboxCluster)
			Expect(err).NotTo(HaveOccurred())
			// Since Initialization is not a pointer, check for the zero value
			Expect(freeboxCluster.Status.Initialization.Provisioned).To(BeNil(), "Status.Initialization.Provisioned should not be set when paused")
		})
	})
})
