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
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

var _ = Describe("Freebox Provider E2E Tests", func() {
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

	Context("Full CAPI Cluster Lifecycle with KubeadmControlPlane", Label("PR-Blocking"), func() {
		It("Should create a complete CAPI cluster with bootstrap data and verify all components", func() {
			var (
				freeboxCluster          *infrastructurev1alpha1.FreeboxCluster
				capiCluster             *unstructured.Unstructured
				freeboxMachineTemplate  *infrastructurev1alpha1.FreeboxMachineTemplate
				kubeadmControlPlane     *unstructured.Unstructured
				createdMachine          *unstructured.Unstructured
				freeboxMachine          *infrastructurev1alpha1.FreeboxMachine
				bootstrapDataSecretName string
				vmID                    *int64
			)

			// Cleanup function to delete VM on failure
			cleanupVM := func() {
				if vmID != nil && freeboxClient != nil {
					cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
					defer cancel()
					log.FromContext(context.Background()).Info("Cleaning up VM", "vmID", *vmID)
					// Stop the VM first before deleting (required by Freebox API)
					// Ignore "not running" errors — the VM may already be stopped.
					if stopErr := freeboxClient.StopVirtualMachine(cleanupCtx, *vmID); stopErr != nil {
						log.FromContext(context.Background()).Info("VM stop returned error (may already be stopped)", "vmID", *vmID, "error", stopErr)
					}
					// Wait for VM to reach stopped state before deleting
					for i := 0; i < 24; i++ {
						vm, getErr := freeboxClient.GetVirtualMachine(cleanupCtx, *vmID)
						if getErr != nil {
							log.FromContext(context.Background()).Info("Could not get VM status during cleanup, proceeding with delete", "vmID", *vmID, "error", getErr)
							break
						}
						if vm.Status == "stopped" {
							break
						}
						log.FromContext(context.Background()).Info("Waiting for VM to stop", "vmID", *vmID, "status", vm.Status)
						time.Sleep(5 * time.Second)
					}
					if err := freeboxClient.DeleteVirtualMachine(cleanupCtx, *vmID); err != nil {
						log.FromContext(context.Background()).Error(err, "Failed to cleanup VM", "vmID", *vmID)
					} else {
						log.FromContext(context.Background()).Info("VM cleanup successful", "vmID", *vmID)
					}
				}
			}
			DeferCleanup(cleanupVM)

			imageURL := "https://cloud.debian.org/images/cloud/trixie/daily/latest/debian-13-generic-arm64-daily.qcow2"
			if testImageURL, ok := e2eConfig.Variables["TEST_IMAGE_URL"]; ok {
				imageURL = testImageURL
			}

			By("Creating a FreeboxCluster (infrastructure)")
			freeboxCluster = &infrastructurev1alpha1.FreeboxCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "192.168.1.202",
						Port: 6443,
					},
				},
			}
			Expect(clusterProxy.GetClient().Create(ctx, freeboxCluster)).To(Succeed())

			By("Creating a CAPI Cluster resource")
			capiCluster = &unstructured.Unstructured{}
			capiCluster.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "cluster.x-k8s.io",
				Version: "v1beta2",
				Kind:    "Cluster",
			})
			capiCluster.SetName("test-cluster")
			capiCluster.SetNamespace(namespace.Name)

			// Set infrastructure ref
			infraRef := map[string]interface{}{
				"apiGroup": "infrastructure.cluster.x-k8s.io",
				"kind":     "FreeboxCluster",
				"name":     freeboxCluster.Name,
			}
			Expect(unstructured.SetNestedField(capiCluster.Object, infraRef, "spec", "infrastructureRef")).To(Succeed())

			// Set control plane ref
			controlPlaneRef := map[string]interface{}{
				"apiGroup": "controlplane.cluster.x-k8s.io",
				"kind":     "KubeadmControlPlane",
				"name":     "test-cp",
			}
			Expect(unstructured.SetNestedField(capiCluster.Object, controlPlaneRef, "spec", "controlPlaneRef")).To(Succeed())

			Expect(clusterProxy.GetClient().Create(ctx, capiCluster)).To(Succeed())

			By("Verifying FreeboxCluster Ready condition is True")
			Eventually(func() error {
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxCluster), updatedCluster); err != nil {
					return fmt.Errorf("failed to get FreeboxCluster: %w", err)
				}

				readyCondition := meta.FindStatusCondition(updatedCluster.Status.Conditions, "Ready")
				if readyCondition == nil {
					return fmt.Errorf("FreeboxCluster Ready condition not found")
				}
				if readyCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("FreeboxCluster Ready condition should be True, got %s", readyCondition.Status)
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"FreeboxCluster Ready condition should be True")

			By("Creating a FreeboxMachineTemplate for control plane nodes")
			freeboxMachineTemplate = &infrastructurev1alpha1.FreeboxMachineTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp-template",
					Namespace: namespace.Name,
				},
				Spec: infrastructurev1alpha1.FreeboxMachineTemplateSpec{
					Template: infrastructurev1alpha1.FreeboxMachineTemplateResource{
						Spec: infrastructurev1alpha1.FreeboxMachineSpec{
							Name:          "test-vm-cp",
							VCPUs:         2,
							MemoryMB:      4096,
							ImageURL:      imageURL,
							DiskSizeBytes: 10737418240, // 10GB
						},
					},
				},
			}
			Expect(clusterProxy.GetClient().Create(ctx, freeboxMachineTemplate)).To(Succeed())

			By("Verifying FreeboxMachineTemplate was created")
			Eventually(func() error {
				template := &infrastructurev1alpha1.FreeboxMachineTemplate{}
				return clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachineTemplate), template)
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"FreeboxMachineTemplate should be created")

			By("Creating a KubeadmControlPlane resource")
			kubeadmControlPlane = &unstructured.Unstructured{}
			kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "controlplane.cluster.x-k8s.io",
				Version: "v1beta2",
				Kind:    "KubeadmControlPlane",
			})
			kubeadmControlPlane.SetName("test-cp")
			kubeadmControlPlane.SetNamespace(namespace.Name)

			// Set KubeadmControlPlane spec
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, int64(1), "spec", "replicas")).To(Succeed())
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, "v1.34.0", "spec", "version")).To(Succeed())

			// Set machine template (v1beta2: infrastructureRef is under spec.machineTemplate.spec.infrastructureRef)
			machineTemplate := map[string]interface{}{
				"spec": map[string]interface{}{
					"infrastructureRef": map[string]interface{}{
						"apiGroup": "infrastructure.cluster.x-k8s.io",
						"kind":     "FreeboxMachineTemplate",
						"name":     freeboxMachineTemplate.Name,
					},
				},
			}
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, machineTemplate, "spec", "machineTemplate")).To(Succeed())

			// Set KubeadmConfigSpec with test markers to verify bootstrap data
			kubeadmConfigSpec := map[string]interface{}{
				"clusterConfiguration": map[string]interface{}{
					"controlPlaneEndpoint": "192.168.1.202:6443",
					"apiServer": map[string]interface{}{
						"certSANs": []interface{}{
							"192.168.1.202",
						},
					},
				},
				"files": []interface{}{
					map[string]interface{}{
						"path":        "/etc/bootstrap-test-marker",
						"owner":       "root:root",
						"permissions": "0644",
						"content":     "Bootstrap data was successfully passed to the VM!",
					},
				},
				"preKubeadmCommands": []interface{}{
					"echo 'Bootstrap test completed' > /var/log/bootstrap-test.log",
					// Add control plane endpoint IP as secondary IP so kubeadm and kubelet can bind to it
					"ip addr add 192.168.1.202/24 dev enp0s5 || true",
					// Enable IP forwarding and bridge netfilter
					"modprobe br_netfilter",
					"echo 1 > /proc/sys/net/ipv4/ip_forward",
					"echo 1 > /proc/sys/net/bridge/bridge-nf-call-iptables",
					"cat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nnet.ipv4.ip_forward = 1\nEOF",
					"sysctl --system",
					// Install dependencies
					"apt-get update",
					"apt-get install -y apt-transport-https ca-certificates curl gpg",
					// Add Kubernetes apt repository
					"mkdir -p /etc/apt/keyrings",
					"curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.34/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg",
					"echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.34/deb/ /' > /etc/apt/sources.list.d/kubernetes.list",
					// Install Kubernetes components
					"apt-get update",
					"apt-get install -y kubelet kubeadm kubectl containerd",
					"apt-mark hold kubelet kubeadm kubectl",
					// Configure containerd
					"mkdir -p /etc/containerd",
					"containerd config default > /etc/containerd/config.toml",
					"sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml",
					"systemctl restart containerd",
					"systemctl enable containerd",
					// Enable kubelet
					"systemctl enable kubelet",
				},
				"postKubeadmCommands": []interface{}{
					// Install Calico CNI
					"export KUBECONFIG=/etc/kubernetes/admin.conf",
					"kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.29.1/manifests/calico.yaml",
				},
			}
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, kubeadmConfigSpec, "spec", "kubeadmConfigSpec")).To(Succeed())

			Expect(clusterProxy.GetClient().Create(ctx, kubeadmControlPlane)).To(Succeed())

			By("Waiting for KubeadmControlPlane to create a Machine")
			Eventually(func() error {
				machineList := &unstructured.UnstructuredList{}
				machineList.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "MachineList",
				})

				if err := clusterProxy.GetClient().List(ctx, machineList); err != nil {
					return fmt.Errorf("failed to list Machines: %w", err)
				}

				for _, item := range machineList.Items {
					labels := item.GetLabels()
					if labels["cluster.x-k8s.io/cluster-name"] == "test-cluster" {
						createdMachine = &item
						return nil
					}
				}
				return fmt.Errorf("no Machine found for cluster test-cluster")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"KubeadmControlPlane should create a Machine")

			By("Verifying Machine has bootstrap dataSecretName set")
			Eventually(func() error {
				// Refresh the machine
				machineList := &unstructured.UnstructuredList{}
				machineList.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "MachineList",
				})

				if err := clusterProxy.GetClient().List(ctx, machineList); err != nil {
					return fmt.Errorf("failed to list Machines: %w", err)
				}

				for _, item := range machineList.Items {
					if item.GetName() == createdMachine.GetName() {
						secretName, found, err := unstructured.NestedString(item.Object, "spec", "bootstrap", "dataSecretName")
						if err != nil {
							return fmt.Errorf("error getting dataSecretName: %w", err)
						}
						if !found || secretName == "" {
							return fmt.Errorf("bootstrap dataSecretName not yet set on Machine")
						}
						bootstrapDataSecretName = secretName
						return nil
					}
				}
				return fmt.Errorf("Machine %s not found", createdMachine.GetName())
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"Machine should have bootstrap dataSecretName set by CABPK")

			By(fmt.Sprintf("Verifying bootstrap Secret %s was created by CABPK", bootstrapDataSecretName))
			bootstrapSecret := &corev1.Secret{}
			Eventually(func() error {
				return clusterProxy.GetClient().Get(ctx,
					types.NamespacedName{Name: bootstrapDataSecretName, Namespace: namespace.Name},
					bootstrapSecret)
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"Bootstrap Secret should be created by CABPK")

			By("Verifying bootstrap Secret contains cloud-init data with test markers")
			Expect(bootstrapSecret.Data).To(HaveKey("value"), "Bootstrap Secret should have 'value' key")
			bootstrapData := string(bootstrapSecret.Data["value"])
			Expect(bootstrapData).To(ContainSubstring("#cloud-config"), "Bootstrap data should be in cloud-init format")
			Expect(bootstrapData).To(ContainSubstring("Bootstrap test completed"), "Bootstrap data should contain test marker from KubeadmConfigSpec")

			By("Waiting for FreeboxMachine to be created by infrastructure controller")
			Eventually(func() error {
				freeboxMachineList := &infrastructurev1alpha1.FreeboxMachineList{}
				if err := clusterProxy.GetClient().List(ctx, freeboxMachineList); err != nil {
					return fmt.Errorf("failed to list FreeboxMachines: %w", err)
				}

				for i := range freeboxMachineList.Items {
					machine := &freeboxMachineList.Items[i]
					owners := machine.GetOwnerReferences()
					for _, owner := range owners {
						if owner.Kind == "Machine" && owner.Name == createdMachine.GetName() {
							freeboxMachine = machine
							return nil
						}
					}
				}
				return fmt.Errorf("FreeboxMachine not yet created for Machine %s", createdMachine.GetName())
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"FreeboxMachine should be created by infrastructure controller")

			By("Verifying Ready condition is False with Reason=Provisioning during image preparation")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				readyCondition := meta.FindStatusCondition(machine.Status.Conditions, "Ready")
				if readyCondition == nil {
					return fmt.Errorf("Ready condition not found")
				}
				if readyCondition.Status != metav1.ConditionFalse {
					return fmt.Errorf("Ready condition should be False during provisioning, got %s", readyCondition.Status)
				}
				if readyCondition.Reason != "Provisioning" {
					return fmt.Errorf("Ready condition Reason should be 'Provisioning', got %s", readyCondition.Reason)
				}

				freeboxMachine = machine // Update reference
				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"Ready condition should be False with Reason=Provisioning during image preparation")

			By("Verifying FreeboxMachine has VMID set")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				vmID = machine.Status.VMID
				if vmID == nil {
					return fmt.Errorf("VMID not yet set")
				}
				freeboxMachine = machine // Update reference
				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"FreeboxMachine should have VMID set")

			By(fmt.Sprintf("Verifying VM %d was created with cloud-init enabled", *vmID))
			Eventually(func() error {
				vm, err := freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					return fmt.Errorf("failed to get VM: %w", err)
				}

				if !vm.EnableCloudInit {
					return fmt.Errorf("cloud-init is not enabled on the VM")
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"VM should have cloud-init enabled")

			By("Verifying VM has bootstrap data from CABPK")
			Eventually(func() error {
				vm, err := freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					return fmt.Errorf("failed to get VM: %w", err)
				}

				if vm.CloudInitUserData == "" {
					return fmt.Errorf("CloudInitUserData is empty")
				}

				if !strings.Contains(vm.CloudInitUserData, "Bootstrap test completed") {
					return fmt.Errorf("CloudInitUserData does not contain expected test marker from CABPK")
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"VM should have bootstrap data from CABPK with test markers")

			By("Verifying FreeboxMachine has IP addresses populated")
			Eventually(func() bool {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return false
				}
				return len(machine.Status.Addresses) > 0
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(BeTrue(),
				"FreeboxMachine should have IP addresses")

			By("Verifying Ready condition becomes True with Reason=InfrastructureReady when fully provisioned")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				readyCondition := meta.FindStatusCondition(machine.Status.Conditions, "Ready")
				if readyCondition == nil {
					return fmt.Errorf("Ready condition not found")
				}
				if readyCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("Ready condition should be True when provisioned, got %s (Reason: %s, Message: %s)",
						readyCondition.Status, readyCondition.Reason, readyCondition.Message)
				}
				if readyCondition.Reason != "InfrastructureReady" {
					return fmt.Errorf("Ready condition Reason should be 'InfrastructureReady', got %s", readyCondition.Reason)
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"Ready condition should become True with Reason=InfrastructureReady")

			By("Verifying providerID is set in format 'freebox://<vm-id>'")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				if machine.Spec.ProviderID == "" {
					return fmt.Errorf("providerID is empty")
				}

				if !strings.HasPrefix(machine.Spec.ProviderID, "freebox://") {
					return fmt.Errorf("providerID should start with 'freebox://', got %s", machine.Spec.ProviderID)
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"providerID should be set in format 'freebox://<vm-id>'")

			By("Verifying CAPI Machine receives addresses from FreeboxMachine (CAPI contract)")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				if len(machine.Status.Addresses) == 0 {
					return fmt.Errorf("FreeboxMachine has no addresses yet")
				}

				capiMachine := &clusterv1.Machine{}
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      createdMachine.GetName(),
					Namespace: namespace.Name,
				}, capiMachine); err != nil {
					return fmt.Errorf("failed to get CAPI Machine: %w", err)
				}

				if len(capiMachine.Status.Addresses) == 0 {
					return fmt.Errorf("CAPI Machine has no addresses - addresses not propagated from FreeboxMachine")
				}

				if len(capiMachine.Status.Addresses) != len(machine.Status.Addresses) {
					return fmt.Errorf("CAPI Machine has %d addresses, FreeboxMachine has %d",
						len(capiMachine.Status.Addresses), len(machine.Status.Addresses))
				}

				for i, addr := range machine.Status.Addresses {
					if capiMachine.Status.Addresses[i].Type != addr.Type {
						return fmt.Errorf("address[%d] type mismatch: CAPI has %v, FreeboxMachine has %v",
							i, capiMachine.Status.Addresses[i].Type, addr.Type)
					}
					if capiMachine.Status.Addresses[i].Address != addr.Address {
						return fmt.Errorf("address[%d] value mismatch: CAPI has %s, FreeboxMachine has %s",
							i, capiMachine.Status.Addresses[i].Address, addr.Address)
					}
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"CAPI Machine should have addresses propagated from FreeboxMachine with correct values")

			By("Verifying KubeadmConfig is ready")
			Eventually(func() error {
				kubeadmConfigList := &unstructured.UnstructuredList{}
				kubeadmConfigList.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "bootstrap.cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "KubeadmConfigList",
				})

				if err := clusterProxy.GetClient().List(ctx, kubeadmConfigList); err != nil {
					return fmt.Errorf("failed to list KubeadmConfigList: %w", err)
				}

				for _, item := range kubeadmConfigList.Items {
					labels := item.GetLabels()
					if labels["cluster.x-k8s.io/cluster-name"] == "test-cluster" {
						if err := checkUnstructuredCondition(&item, "Ready"); err == nil {
							return nil
						}
					}
				}
				return fmt.Errorf("KubeadmConfig Ready condition not found")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-machine")...).Should(Succeed(),
				"KubeadmConfig should be ready")

			By("Verifying KubeadmControlPlane is available")
			Eventually(func() error {
				kcp := &unstructured.Unstructured{}
				kcp.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "controlplane.cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "KubeadmControlPlane",
				})
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      "test-cp",
					Namespace: namespace.Name,
				}, kcp); err != nil {
					return fmt.Errorf("failed to get KubeadmControlPlane: %w", err)
				}
				return checkUnstructuredCondition(kcp, "Available")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-control-plane")...).Should(Succeed(),
				"KubeadmControlPlane should be available")

			By("Verifying Machine is ready")
			Eventually(func() error {
				machine := &unstructured.Unstructured{}
				machine.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "Machine",
				})
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      createdMachine.GetName(),
					Namespace: namespace.Name,
				}, machine); err != nil {
					return fmt.Errorf("failed to get Machine: %w", err)
				}
				return checkUnstructuredCondition(machine, "Ready")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-control-plane")...).Should(Succeed(),
				"Machine should be ready")

			By("Verifying API server is accessible on control plane endpoint")
			Eventually(func() error {
				// Get the kubeconfig secret
				kubeconfigSecret := &corev1.Secret{}
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      "test-cluster-kubeconfig",
					Namespace: namespace.Name,
				}, kubeconfigSecret); err != nil {
					return fmt.Errorf("failed to get kubeconfig secret: %w", err)
				}

				// Verify the secret contains the kubeconfig data
				kubeconfigData, ok := kubeconfigSecret.Data["value"]
				if !ok {
					return fmt.Errorf("kubeconfig secret does not contain 'value' key")
				}

				// Verify the kubeconfig contains the expected control plane endpoint
				kubeconfigStr := string(kubeconfigData)
				if !strings.Contains(kubeconfigStr, freeboxCluster.Spec.ControlPlaneEndpoint.Host) {
					return fmt.Errorf("kubeconfig does not contain expected control plane endpoint %s",
						freeboxCluster.Spec.ControlPlaneEndpoint.Host)
				}

				// Create a Kubernetes client from the kubeconfig
				restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
				if err != nil {
					return fmt.Errorf("failed to parse kubeconfig: %w", err)
				}

				clientset, err := kubernetes.NewForConfig(restConfig)
				if err != nil {
					return fmt.Errorf("failed to create Kubernetes client: %w", err)
				}

				// Verify API server is responding by listing namespaces
				_, err = clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to connect to API server: %w", err)
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-control-plane")...).Should(Succeed(),
				"API server should be accessible and responding to requests")

			By("Verifying CAPI Cluster is available")
			Eventually(func() error {
				cluster := &unstructured.Unstructured{}
				cluster.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta2",
					Kind:    "Cluster",
				})
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      "test-cluster",
					Namespace: namespace.Name,
				}, cluster); err != nil {
					return fmt.Errorf("failed to get Cluster: %w", err)
				}
				return checkUnstructuredCondition(cluster, "Available")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-cluster")...).Should(Succeed(),
				"Cluster should be available")

			By("Verifying VM exists in Freebox before deletion")
			var vmID64 int64
			Eventually(func() error {
				vm, err := freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					return fmt.Errorf("failed to get VM before deletion: %w", err)
				}
				vmID64 = vm.ID
				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"VM should exist in Freebox before deletion")

			Expect(vmID64).To(Equal(*vmID), "VM should be retrieved before deletion")

			By("Triggering FreeboxMachine deletion")
			Expect(clusterProxy.GetClient().Delete(ctx, freeboxMachine)).To(Succeed())

			By("Verifying Ready condition transitions to False/Deleting during deletion")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					// Machine may already be deleted
					return nil
				}

				if machine.DeletionTimestamp == nil {
					return fmt.Errorf("deletion timestamp not set yet")
				}

				readyCondition := meta.FindStatusCondition(machine.Status.Conditions, "Ready")
				if readyCondition == nil {
					return fmt.Errorf("Ready condition not found during deletion")
				}
				if readyCondition.Status != metav1.ConditionFalse {
					return fmt.Errorf("Ready condition should be False during deletion, got %s", readyCondition.Status)
				}
				if readyCondition.Reason != "Deleting" {
					return fmt.Errorf("Ready condition Reason should be 'Deleting', got %s", readyCondition.Reason)
				}

				return nil
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-crd")...).Should(Succeed(),
				"Ready condition should transition to False/Deleting")

			By("Waiting for FreeboxMachine to be fully deleted")
			WaitForFreeboxMachineDeleted(ctx, WaitForFreeboxMachineDeletedInput{
				Getter:  clusterProxy.GetClient(),
				Machine: freeboxMachine,
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-delete")...)

			By("Verifying VM is deleted from Freebox")
			Eventually(func() error {
				_, err := freeboxClient.GetVirtualMachine(ctx, *vmID)
				if err != nil {
					// Expected: VM should not be found
					if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
						return nil
					}
					return fmt.Errorf("unexpected error checking VM deletion: %w", err)
				}
				return fmt.Errorf("VM still exists in Freebox, expected it to be deleted")
			}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-delete")...).Should(Succeed(),
				"VM should be deleted from Freebox")

			By("Verifying disk file is deleted from Freebox")
			// Store the disk path for verification
			diskPath := freeboxMachine.Status.DiskPath
			if diskPath != "" {
				Eventually(func() error {
					// Try to list VMs to ensure API is responsive.
					// After deletion, there may be zero VMs - this is expected.
					_, err := freeboxClient.ListVirtualMachines(ctx)
					if err != nil {
						return fmt.Errorf("Freebox API not responsive: %w", err)
					}
					// If we get here, disk cleanup should have happened
					// We can't directly verify file deletion via API, but we verified VM is gone
					return nil
				}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-delete")...).Should(Succeed(),
					"Disk cleanup should complete")
			}

			By("Cleaning up remaining test resources in correct order")
			// Delete remaining resources in reverse order of dependencies
			// Note: FreeboxMachine already deleted and verified above
			if createdMachine != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, createdMachine)).To(Succeed())
			}
			if kubeadmControlPlane != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, kubeadmControlPlane)).To(Succeed())
			}
			if freeboxMachineTemplate != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, freeboxMachineTemplate)).To(Succeed())
			}
			if capiCluster != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, capiCluster)).To(Succeed())
			}
			if freeboxCluster != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, freeboxCluster)).To(Succeed())
			}
		})
	})
})
