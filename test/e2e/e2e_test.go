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

			imageURL := "https://cloud.debian.org/images/cloud/trixie/daily/latest/debian-13-nocloud-arm64-daily.qcow2"
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
			}, e2eConfig.GetIntervals("default", "wait-crd")...) // Use shorter timeout just to check it exists

			By("Verifying the image has been properly downloaded and VM created")
			var vmID *int64
			Eventually(func() error {
				// Re-fetch the machine to get the latest status
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				key := GetObjectKey(createdMachine)
				if err := clusterProxy.GetClient().Get(ctx, key, machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				// First check if image file exists
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
				expectedFileName := machine.Spec.Name + ext
				imagePath := path.Join(vmStoragePath, expectedFileName)

				fileInfo, err := freeboxClient.GetFileInfo(ctx, imagePath)
				if err != nil {
					return fmt.Errorf("VM image file not yet available at %s: %w", imagePath, err)
				}

				// Verify it's a file and has reasonable size
				if fileInfo.SizeBytes == 0 {
					return fmt.Errorf("VM image file %s exists but has zero size", imagePath)
				}

				// Now check if VM has been created
				vmID = machine.Status.VMID
				if vmID == nil {
					return fmt.Errorf("VMID not yet set in FreeboxMachine status (image ready, waiting for VM creation)")
				}

				// Verify the VM exists in Freebox
				_, err = freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					return fmt.Errorf("failed to get VM with ID %d from Freebox: %w", *vmID, err)
				}

				return nil
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"Image and VM should be created for FreeboxMachine %s/%s", createdMachine.Namespace, createdMachine.Name)

			By("Deleting the FreeboxMachine")
			Expect(clusterProxy.GetClient().Delete(ctx, machine)).To(Succeed())

			By("Waiting for the FreeboxMachine to be deleted")
			WaitForFreeboxMachineDeleted(ctx, WaitForFreeboxMachineDeletedInput{
				Getter:  clusterProxy.GetClient(),
				Machine: machine,
			}, e2eConfig.GetIntervals("default", "wait-delete")...)

			By("Verifying the VM has been destroyed on the Freebox")
			Eventually(func() error {
				// Verify the VM no longer exists in Freebox
				_, err := freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					// VM not found is expected after deletion
					return nil
				}
				return fmt.Errorf("VM with ID %d still exists on Freebox after FreeboxMachine deletion", *vmID)
			}, e2eConfig.GetIntervals("default", "wait-delete")...).Should(Succeed(),
				"VM should be destroyed on Freebox after FreeboxMachine deletion")
		})
	})

	Context("FreeboxCluster Lifecycle", Label("PR-Blocking"), func() {
		It("Should create and delete a FreeboxCluster successfully", func() {
			By("Creating a FreeboxCluster resource")
			cluster := &infrastructurev1alpha1.FreeboxCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxClusterSpec{},
			}
			Expect(clusterProxy.GetClient().Create(ctx, cluster)).To(Succeed())

			clusterKey := GetObjectKey(cluster)

			By("Waiting for the FreeboxCluster to be marked as provisioned")
			Eventually(func() bool {
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				err := clusterProxy.GetClient().Get(ctx, clusterKey, updatedCluster)
				if err != nil {
					return false
				}
				return updatedCluster.Status.Initialization.Provisioned != nil &&
					*updatedCluster.Status.Initialization.Provisioned
			}, e2eConfig.GetIntervals("default", "wait-vm-start")...).Should(BeTrue())

			By("Verifying the FreeboxCluster has a Ready condition")
			updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
			Expect(clusterProxy.GetClient().Get(ctx, clusterKey, updatedCluster)).To(Succeed())

			// Check for Ready condition
			hasReadyCondition := false
			for _, condition := range updatedCluster.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
					hasReadyCondition = true
					break
				}
			}
			Expect(hasReadyCondition).To(BeTrue(), "FreeboxCluster should have a Ready=True condition")

			By("Deleting the FreeboxCluster")
			Expect(clusterProxy.GetClient().Delete(ctx, cluster)).To(Succeed())

			By("Waiting for the FreeboxCluster to be deleted")
			Eventually(func() bool {
				err := clusterProxy.GetClient().Get(ctx, clusterKey,
					&infrastructurev1alpha1.FreeboxCluster{})
				return err != nil
			}, e2eConfig.GetIntervals("default", "wait-delete")...).Should(BeTrue())
		})

		It("Should work with FreeboxMachines in the same namespace", func() {
			By("Creating a FreeboxCluster resource")
			cluster := &infrastructurev1alpha1.FreeboxCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-with-machine",
					Namespace: namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxClusterSpec{},
			}
			Expect(clusterProxy.GetClient().Create(ctx, cluster)).To(Succeed())

			clusterKey := GetObjectKey(cluster)

			By("Waiting for the FreeboxCluster to be provisioned")
			Eventually(func() bool {
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				err := clusterProxy.GetClient().Get(ctx, clusterKey, updatedCluster)
				if err != nil {
					return false
				}
				return updatedCluster.Status.Initialization.Provisioned != nil &&
					*updatedCluster.Status.Initialization.Provisioned
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(BeTrue())

			By("Creating a FreeboxMachine resource with cluster label")
			imageURL := "https://cloud.debian.org/images/cloud/trixie/daily/latest/debian-13-nocloud-arm64-daily.qcow2"
			machine := &infrastructurev1alpha1.FreeboxMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm-for-cluster",
					Namespace: namespace.Name,
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": cluster.Name,
					},
				},
				Spec: infrastructurev1alpha1.FreeboxMachineSpec{
					Name:          "test-vm-for-cluster",
					ImageURL:      imageURL,
					VCPUs:         1,
					MemoryMB:      2048,
					DiskSizeBytes: 10 * 1024 * 1024 * 1024, // 10GB
				},
			}
			Expect(clusterProxy.GetClient().Create(ctx, machine)).To(Succeed())

			machineKey := GetObjectKey(machine)

			By("Verifying the FreeboxMachine can be created alongside the cluster")
			Eventually(func() bool {
				updatedMachine := &infrastructurev1alpha1.FreeboxMachine{}
				err := clusterProxy.GetClient().Get(ctx, machineKey, updatedMachine)
				return err == nil
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(BeTrue())

			By("Deleting the FreeboxMachine")
			Expect(clusterProxy.GetClient().Delete(ctx, machine)).To(Succeed())

			By("Waiting for the FreeboxMachine to be deleted")
			Eventually(func() bool {
				err := clusterProxy.GetClient().Get(ctx, machineKey,
					&infrastructurev1alpha1.FreeboxMachine{})
				return err != nil
			}, e2eConfig.GetIntervals("default", "wait-delete")...).Should(BeTrue())

			By("Deleting the FreeboxCluster")
			Expect(clusterProxy.GetClient().Delete(ctx, cluster)).To(Succeed())

			By("Waiting for the FreeboxCluster to be deleted")
			Eventually(func() bool {
				err := clusterProxy.GetClient().Get(ctx, clusterKey,
					&infrastructurev1alpha1.FreeboxCluster{})
				return err != nil
			}, e2eConfig.GetIntervals("default", "wait-delete")...).Should(BeTrue())
		})
	})

	Context("FreeboxMachineTemplate", Label("PR-Blocking"), func() {
		It("Should have FreeboxMachineTemplate CRD available", func() {
			By("Listing FreeboxMachineTemplate resources")
			templateList := &infrastructurev1alpha1.FreeboxMachineTemplateList{}
			Eventually(func() error {
				return clusterProxy.GetClient().List(ctx, templateList)
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed())
		})

		It("Should create and delete a FreeboxMachineTemplate successfully", func() {
			By("Creating a FreeboxMachineTemplate resource")
			template := &infrastructurev1alpha1.FreeboxMachineTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxMachineTemplateSpec{
					Template: infrastructurev1alpha1.FreeboxMachineTemplateResource{
						Spec: infrastructurev1alpha1.FreeboxMachineSpec{
							Name:          "test-vm-from-template",
							VCPUs:         2,
							MemoryMB:      4096,
							ImageURL:      "https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/v1.11.5/nocloud-arm64.raw.xz",
							DiskSizeBytes: 21474836480, // 20GB
						},
					},
				},
			}
			Expect(clusterProxy.GetClient().Create(ctx, template)).To(Succeed())

			templateKey := GetObjectKey(template)

			By("Verifying the FreeboxMachineTemplate was created")
			Eventually(func() error {
				return clusterProxy.GetClient().Get(ctx, templateKey, template)
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed())

			By("Deleting the FreeboxMachineTemplate")
			Expect(clusterProxy.GetClient().Delete(ctx, template)).To(Succeed())

			By("Waiting for the FreeboxMachineTemplate to be deleted")
			Eventually(func() bool {
				err := clusterProxy.GetClient().Get(ctx, templateKey,
					&infrastructurev1alpha1.FreeboxMachineTemplate{})
				return err != nil
			}, e2eConfig.GetIntervals("default", "wait-delete")...).Should(BeTrue())
		})
	})
})
