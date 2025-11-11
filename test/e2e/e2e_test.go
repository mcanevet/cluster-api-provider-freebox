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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

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
				Version: "v1beta1",
				Kind:    "Cluster",
			})
			capiCluster.SetName("test-cluster")
			capiCluster.SetNamespace(namespace.Name)

			// Set infrastructure ref
			infraRef := map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
				"kind":       "FreeboxCluster",
				"name":       freeboxCluster.Name,
			}
			Expect(unstructured.SetNestedField(capiCluster.Object, infraRef, "spec", "infrastructureRef")).To(Succeed())

			// Set control plane ref
			controlPlaneRef := map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "KubeadmControlPlane",
				"name":       "test-cp",
			}
			Expect(unstructured.SetNestedField(capiCluster.Object, controlPlaneRef, "spec", "controlPlaneRef")).To(Succeed())

			Expect(clusterProxy.GetClient().Create(ctx, capiCluster)).To(Succeed())

			By("Verifying FreeboxCluster is provisioned")
			Eventually(func() bool {
				updatedCluster := &infrastructurev1alpha1.FreeboxCluster{}
				err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxCluster), updatedCluster)
				if err != nil {
					return false
				}
				return updatedCluster.Status.Initialization.Provisioned != nil &&
					*updatedCluster.Status.Initialization.Provisioned
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(BeTrue(),
				"FreeboxCluster should be provisioned")

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
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed(),
				"FreeboxMachineTemplate should be created")

			By("Creating a KubeadmControlPlane resource")
			kubeadmControlPlane = &unstructured.Unstructured{}
			kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "controlplane.cluster.x-k8s.io",
				Version: "v1beta1",
				Kind:    "KubeadmControlPlane",
			})
			kubeadmControlPlane.SetName("test-cp")
			kubeadmControlPlane.SetNamespace(namespace.Name)

			// Set KubeadmControlPlane spec
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, int64(1), "spec", "replicas")).To(Succeed())
			Expect(unstructured.SetNestedField(kubeadmControlPlane.Object, "v1.34.0", "spec", "version")).To(Succeed())

			// Set machine template
			machineTemplate := map[string]interface{}{
				"infrastructureRef": map[string]interface{}{
					"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
					"kind":       "FreeboxMachineTemplate",
					"name":       freeboxMachineTemplate.Name,
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
					// Add control plane endpoint IP as secondary IP
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
					Version: "v1beta1",
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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"KubeadmControlPlane should create a Machine")

			By("Verifying Machine has bootstrap dataSecretName set")
			Eventually(func() error {
				// Refresh the machine
				machineList := &unstructured.UnstructuredList{}
				machineList.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta1",
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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"Machine should have bootstrap dataSecretName set by CABPK")

			By(fmt.Sprintf("Verifying bootstrap Secret %s was created by CABPK", bootstrapDataSecretName))
			bootstrapSecret := &corev1.Secret{}
			Eventually(func() error {
				return clusterProxy.GetClient().Get(ctx,
					types.NamespacedName{Name: bootstrapDataSecretName, Namespace: namespace.Name},
					bootstrapSecret)
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed(),
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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"FreeboxMachine should be created by infrastructure controller")

			By("Verifying Ready condition is False with Reason=Provisioning during image preparation")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				// Find the Ready condition
				var readyCondition *metav1.Condition
				for i := range machine.Status.Conditions {
					if machine.Status.Conditions[i].Type == "Ready" {
						readyCondition = &machine.Status.Conditions[i]
						break
					}
				}

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
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed(),
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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
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
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed(),
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
			}, e2eConfig.GetIntervals("default", "wait-crd")...).Should(Succeed(),
				"VM should have bootstrap data from CABPK with test markers")

			By("Verifying FreeboxMachine has IP addresses populated")
			Eventually(func() bool {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return false
				}
				return len(machine.Status.Addresses) > 0
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(BeTrue(),
				"FreeboxMachine should have IP addresses")

			By("Verifying Ready condition becomes True with Reason=InfrastructureReady when fully provisioned")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				// Find the Ready condition
				var readyCondition *metav1.Condition
				for i := range machine.Status.Conditions {
					if machine.Status.Conditions[i].Type == "Ready" {
						readyCondition = &machine.Status.Conditions[i]
						break
					}
				}

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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"Ready condition should become True with Reason=InfrastructureReady")

			By("Verifying initialization.provisioned is set to true")
			Eventually(func() error {
				machine := &infrastructurev1alpha1.FreeboxMachine{}
				if err := clusterProxy.GetClient().Get(ctx, GetObjectKey(freeboxMachine), machine); err != nil {
					return fmt.Errorf("failed to get FreeboxMachine: %w", err)
				}

				if machine.Status.Initialization.Provisioned == nil {
					return fmt.Errorf("initialization.provisioned is nil")
				}

				if !*machine.Status.Initialization.Provisioned {
					return fmt.Errorf("initialization.provisioned should be true")
				}

				return nil
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"initialization.provisioned should be true")

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
			}, e2eConfig.GetIntervals("default", "wait-machine")...).Should(Succeed(),
				"providerID should be set in format 'freebox://<vm-id>'")

			By("Waiting for CAPI Cluster to be ready")
			Eventually(func() bool {
				cluster := &unstructured.Unstructured{}
				cluster.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "cluster.x-k8s.io",
					Version: "v1beta1",
					Kind:    "Cluster",
				})
				if err := clusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Name:      "test-cluster",
					Namespace: namespace.Name,
				}, cluster); err != nil {
					return false
				}

				// Check if cluster is ready via status.phase
				phase, found, err := unstructured.NestedString(cluster.Object, "status", "phase")
				if err != nil || !found {
					return false
				}
				return phase == "Provisioned"
			}, e2eConfig.GetIntervals("default", "wait-cluster")...).Should(BeTrue(),
				"Cluster should become ready")

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

				// TODO: Use the kubeconfig to verify API server connectivity
				// For now, just verify the secret exists
				if _, ok := kubeconfigSecret.Data["value"]; !ok {
					return fmt.Errorf("kubeconfig secret does not contain 'value' key")
				}
				return nil
			}, e2eConfig.GetIntervals("default", "wait-control-plane")...).Should(Succeed(),
				"API server should be accessible")

			By("Cleaning up test resources in correct order")
			// Delete in reverse order of dependencies
			if freeboxMachine != nil {
				Expect(clusterProxy.GetClient().Delete(ctx, freeboxMachine)).To(Succeed())
				WaitForFreeboxMachineDeleted(ctx, WaitForFreeboxMachineDeletedInput{
					Getter:  clusterProxy.GetClient(),
					Machine: freeboxMachine,
				}, e2eConfig.GetIntervals("default", "wait-delete")...)
			}
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
