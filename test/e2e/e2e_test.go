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
	"path"

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

			// vmStoragePath is set from the Freebox /system/ API in the suite setup
			vmStoragePath, ok := e2eConfig.Variables["VM_STORAGE_PATH"]
			Expect(ok).To(BeTrue(), "VM_STORAGE_PATH should be set by suite from /system/ user_main_storage")

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
			createdMachine := GetFreeboxMachine(ctx, GetFreeboxMachineInput{
				Getter:    clusterProxy.GetClient(),
				Name:      machineKey.Name,
				Namespace: machineKey.Namespace,
			}, e2eConfig.GetIntervals("default", "wait-machine")...)

			By("Verifying the image has been properly downloaded using Freebox API")
			Eventually(func() error {
				// Verify the file actually exists on Freebox storage with VM-named filename
				// The final image should be named after the VM (machine.Spec.Name) with the underlying disk extension
				// For compressed images (.raw.xz, .img.gz, etc.), we strip the compression suffix
				// For non-compressed images, we keep the original extension
				sourceName := path.Base(imageURL)

				// Determine the underlying extension (without compression suffix)
				underlyingName := sourceName
				compressedExts := []string{".xz", ".gz", ".bz2", ".zip", ".tar"}
				for _, ext := range compressedExts {
					if len(underlyingName) > len(ext) && underlyingName[len(underlyingName)-len(ext):] == ext {
						underlyingName = underlyingName[:len(underlyingName)-len(ext)]
						break
					}
				}

				// Get the extension from the underlying name
				ext := path.Ext(underlyingName)
				if ext == "" {
					ext = ".raw" // Default if no extension found
				}

				// Expected filename is VM name + extension
				expectedFileName := createdMachine.Spec.Name + ext
				imagePath := path.Join(vmStoragePath, expectedFileName)

				fileInfo, err := freeboxClient.GetFileInfo(ctx, imagePath)
				if err != nil {
					return fmt.Errorf("failed to get file info for VM-named image %s: %w", imagePath, err)
				}

				// Verify it's a file and has reasonable size
				if fileInfo.SizeBytes == 0 {
					return fmt.Errorf("VM-named image file %s exists but has zero size", imagePath)
				}

				return nil
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"VM-named image should be downloaded and present on Freebox storage for FreeboxMachine %s/%s",
				createdMachine.Namespace, createdMachine.Name)

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
