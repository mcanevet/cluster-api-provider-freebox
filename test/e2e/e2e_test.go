//go:build e2e
// +build e2e

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

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

var _ = Describe("Freebox Provider Basic Tests", func() {
	var (
		namespace *corev1.Namespace
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "E2E config is required")
		Expect(clusterProxy).ToNot(BeNil(), "Cluster proxy is required")

		By("Creating a namespace for the test")
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "freebox-e2e-",
			},
		}
		Expect(clusterProxy.GetClient().Create(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		if !skipCleanup && namespace != nil {
			By(fmt.Sprintf("Deleting namespace %s", namespace.Name))
			Expect(clusterProxy.GetClient().Delete(ctx, namespace)).To(Succeed())
		}
	})

	Context("CRD Installation", func() {
		It("Should have FreeboxMachine CRD available", func() {
			By("Listing FreeboxMachine resources")
			machineList := &infrastructurev1alpha1.FreeboxMachineList{}
			Eventually(func() error {
				return clusterProxy.GetClient().List(ctx, machineList)
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed())
		})

		It("Should have FreeboxCluster CRD available", func() {
			By("Listing FreeboxCluster resources")
			clusterList := &infrastructurev1alpha1.FreeboxClusterList{}
			Eventually(func() error {
				return clusterProxy.GetClient().List(ctx, clusterList)
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed())
		})
	})

	Context("FreeboxMachine Lifecycle", Label("PR-Blocking"), func() {
		It("Should create and delete a FreeboxMachine successfully", func() {
			By("Creating a FreeboxMachine resource")

			imageURL := "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
			if testImageURL, ok := e2eConfig.Variables["TEST_IMAGE_URL"]; ok {
				imageURL = testImageURL
			}

			machine := &infrastructurev1alpha1.FreeboxMachine{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-machine-",
					Namespace:    namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxMachineSpec{
					Name:          "test-vm",
					VCPUs:         1,
					MemoryMB:      2048,
					ImageURL:      imageURL,
					DiskSizeBytes: 10737418240, // 10GB
				},
			}

			Expect(clusterProxy.GetClient().Create(ctx, machine)).To(Succeed())
			machineKey := GetObjectKey(machine)

			By("Waiting for the FreeboxMachine to be created")
			GetFreeboxMachine(ctx, GetFreeboxMachineInput{
				Getter:    clusterProxy.GetClient(),
				Name:      machineKey.Name,
				Namespace: machineKey.Namespace,
			}, e2eConfig.GetIntervals("default", "wait-machine")...)

			By("Deleting the FreeboxMachine")
			Expect(clusterProxy.GetClient().Delete(ctx, machine)).To(Succeed())

			By("Waiting for the FreeboxMachine to be deleted")
			WaitForFreeboxMachineDeleted(ctx, WaitForFreeboxMachineDeletedInput{
				Getter:  clusterProxy.GetClient(),
				Machine: machine,
			}, e2eConfig.GetIntervals("default", "wait-delete")...)
		})
	})
})
