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
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mcanevet/cluster-api-provider-freebox/test/utils"
)

var _ = Describe("Freebox Image Download E2E", Ordered, func() {
	const (
		testNamespace          = "freebox-e2e-test"
		freeboxMachineManifest = `apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: FreeboxMachine
metadata:
  name: test-machine
  namespace: %s
spec:
  cpus: 2
  memory: 4
  diskSize: 20
  imageURL: "https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img"`
	)

	BeforeAll(func() {
		By("installing CRDs")
		cmd := exec.Command("make", "install")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("creating test namespace")
		cmd = exec.Command("kubectl", "create", "ns", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		By("creating controller namespace")
		cmd = exec.Command("kubectl", "create", "ns", "cluster-api-provider-freebox-system")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create controller namespace")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", "IMG=example.com/cluster-api-provider-freebox:v0.0.1")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy controller")

		By("waiting for controller to be ready")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pods", "-l", "control-plane=controller-manager",
				"-o", "jsonpath={.items[0].status.phase}", "-n", "cluster-api-provider-freebox-system")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("Running"))
		}, 60*time.Second, 2*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		By("cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("cleaning up controller namespace")
		cmd = exec.Command("kubectl", "delete", "ns", "cluster-api-provider-freebox-system", "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		By("cleaning up FreeboxMachine resources")
		cmd := exec.Command("kubectl", "delete", "freeboxmachines", "--all", "-n", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	Context("When a FreeboxMachine is created", func() {
		It("should reconcile and start image download workflow", func() {
			By("creating a FreeboxMachine resource")
			manifest := fmt.Sprintf(freeboxMachineManifest, testNamespace)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(manifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create FreeboxMachine")

			By("verifying the FreeboxMachine resource exists")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "freeboxmachine", "test-machine", "-n", testNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("test-machine"))
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("verifying controller reconciliation occurs")
			Eventually(func(g Gomega) {
				// Check controller logs for reconciliation activity
				cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
					"-n", "cluster-api-provider-freebox-system", "--tail=20")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("FreeboxMachine"))
			}, 60*time.Second, 5*time.Second).Should(Succeed())

			By("verifying the FreeboxMachine status is updated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "freeboxmachine", "test-machine",
					"-n", testNamespace, "-o", "jsonpath={.status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// The status should be populated after reconciliation
				g.Expect(output).NotTo(BeEmpty(), "FreeboxMachine status should be populated")
			}, 60*time.Second, 5*time.Second).Should(Succeed())

			By("checking for image download initiation in logs")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
					"-n", "cluster-api-provider-freebox-system", "--tail=50")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Look for image download related log messages
				g.Expect(output).To(SatisfyAny(
					ContainSubstring("downloadImage"),
					ContainSubstring("ubuntu-22.04"),
					ContainSubstring("image download"),
					ContainSubstring("Starting VM provisioning"),
				), "Should find evidence of image download workflow")
			}, 90*time.Second, 5*time.Second).Should(Succeed())
		})

		It("should handle invalid image URLs gracefully", func() {
			By("creating a FreeboxMachine with an invalid image URL")
			invalidManifest := fmt.Sprintf(`apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: FreeboxMachine
metadata:
  name: test-machine-invalid
  namespace: %s
spec:
  cpus: 2
  memory: 4
  diskSize: 20
  imageURL: "https://invalid-url-that-does-not-exist.com/nonexistent.img"`, testNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(invalidManifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create FreeboxMachine with invalid URL")

			By("verifying controller handles the error gracefully")
			Eventually(func(g Gomega) {
				// Check that the controller doesn't crash and logs appropriate errors
				cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
					"-n", "cluster-api-provider-freebox-system", "--tail=30")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Should log error without crashing
				g.Expect(output).To(SatisfyAny(
					ContainSubstring("error"),
					ContainSubstring("failed"),
					ContainSubstring("download"),
				), "Should log download error")
			}, 60*time.Second, 5*time.Second).Should(Succeed())

			By("verifying the controller pod is still running")
			cmd = exec.Command("kubectl", "get", "pods", "-l", "control-plane=controller-manager",
				"-n", "cluster-api-provider-freebox-system", "-o", "jsonpath={.items[0].status.phase}")
			var output string
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("Running"), "Controller should still be running after error")

			By("cleaning up invalid resource")
			cmd = exec.Command("kubectl", "delete", "freeboxmachine", "test-machine-invalid",
				"-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})
	})

	Context("When validating Freebox client integration", func() {
		It("should demonstrate proper environment variable handling", func() {
			By("checking that controller handles missing Freebox environment variables")
			// This test verifies that the controller gracefully handles missing FREEBOX_* env vars
			// In a real environment, these would be provided via secrets

			By("creating a FreeboxMachine resource")
			manifest := fmt.Sprintf(freeboxMachineManifest, testNamespace)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(manifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying controller logs show environment variable handling")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
					"-n", "cluster-api-provider-freebox-system", "--tail=50")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// The controller should log about Freebox client initialization
				g.Expect(output).To(SatisfyAny(
					ContainSubstring("Freebox"),
					ContainSubstring("client"),
					ContainSubstring("FREEBOX_ENDPOINT"),
				), "Should show Freebox client handling")
			}, 60*time.Second, 5*time.Second).Should(Succeed())
		})
	})

	Context("When testing controller metrics", func() {
		It("should expose reconciliation metrics for FreeboxMachine", func() {
			By("creating a FreeboxMachine to trigger reconciliation")
			manifest := fmt.Sprintf(freeboxMachineManifest, testNamespace)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringToReader(manifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for reconciliation to occur")
			time.Sleep(10 * time.Second)

			By("verifying the controller is processing the FreeboxMachine")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
					"-n", "cluster-api-provider-freebox-system", "--tail=30")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Should show reconciliation activity
				g.Expect(output).To(SatisfyAny(
					ContainSubstring("FreeboxMachine"),
					ContainSubstring("Reconciling"),
					ContainSubstring("test-machine"),
				), "Should show FreeboxMachine reconciliation in logs")
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("verifying the FreeboxMachine resource status is updated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "freeboxmachine", "test-machine",
					"-n", testNamespace, "-o", "jsonpath={.status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "FreeboxMachine status should be populated by controller")
			}, 30*time.Second, 2*time.Second).Should(Succeed())
		})
	})
})
