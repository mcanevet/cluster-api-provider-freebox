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
	"crypto/rsa"
	"fmt"
	"io"

	freeboxclient "github.com/nikolalohinski/free-go/client"
	freeboxTypes "github.com/nikolalohinski/free-go/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

// fakeClient is a minimal fake implementation of freeboxclient.Client for phase-transition tests.
// Only the methods exercised by the reconciler's image-phase logic are implemented;
// all others panic to surface unexpected calls during testing.
type fakeClient struct {
	listDownloadTasksFn  func(ctx context.Context) ([]freeboxTypes.DownloadTask, error)
	addDownloadTaskFn    func(ctx context.Context, req freeboxTypes.DownloadRequest) (int64, error)
	getDownloadTaskFn    func(ctx context.Context, id int64) (freeboxTypes.DownloadTask, error)
	deleteDownloadTaskFn func(ctx context.Context, id int64) error
	extractFileFn        func(ctx context.Context, p freeboxTypes.ExtractFilePayload) (freeboxTypes.FileSystemTask, error)
	copyFilesFn          func(ctx context.Context, srcs []string, dst string, mode freeboxTypes.FileCopyMode) (freeboxTypes.FileSystemTask, error)
	moveFilesFn          func(ctx context.Context, srcs []string, dst string, mode freeboxTypes.FileMoveMode) (freeboxTypes.FileSystemTask, error)
	getFileSystemTaskFn  func(ctx context.Context, id int64) (freeboxTypes.FileSystemTask, error)
	removeFilesFn        func(ctx context.Context, paths []string) (freeboxTypes.FileSystemTask, error)
	resizeVirtualDiskFn  func(ctx context.Context, p freeboxTypes.VirtualDisksResizePayload) (int64, error)
	getVirtualDiskTaskFn func(ctx context.Context, id int64) (freeboxTypes.VirtualMachineDiskTask, error)
	getVirtualMachineFn  func(ctx context.Context, id int64) (freeboxTypes.VirtualMachine, error)
	getLanInterfaceFn    func(ctx context.Context, name string) ([]freeboxTypes.LanInterfaceHost, error)
}

func (f *fakeClient) ListDownloadTasks(ctx context.Context) ([]freeboxTypes.DownloadTask, error) {
	if f.listDownloadTasksFn != nil {
		return f.listDownloadTasksFn(ctx)
	}
	return nil, nil
}
func (f *fakeClient) AddDownloadTask(ctx context.Context, req freeboxTypes.DownloadRequest) (int64, error) {
	if f.addDownloadTaskFn != nil {
		return f.addDownloadTaskFn(ctx, req)
	}
	panic("AddDownloadTask not expected")
}
func (f *fakeClient) GetDownloadTask(ctx context.Context, id int64) (freeboxTypes.DownloadTask, error) {
	if f.getDownloadTaskFn != nil {
		return f.getDownloadTaskFn(ctx, id)
	}
	panic("GetDownloadTask not expected")
}
func (f *fakeClient) DeleteDownloadTask(ctx context.Context, id int64) error {
	if f.deleteDownloadTaskFn != nil {
		return f.deleteDownloadTaskFn(ctx, id)
	}
	return nil
}
func (f *fakeClient) ExtractFile(ctx context.Context, p freeboxTypes.ExtractFilePayload) (freeboxTypes.FileSystemTask, error) {
	if f.extractFileFn != nil {
		return f.extractFileFn(ctx, p)
	}
	panic("ExtractFile not expected")
}
func (f *fakeClient) CopyFiles(ctx context.Context, srcs []string, dst string, mode freeboxTypes.FileCopyMode) (freeboxTypes.FileSystemTask, error) {
	if f.copyFilesFn != nil {
		return f.copyFilesFn(ctx, srcs, dst, mode)
	}
	panic("CopyFiles not expected")
}
func (f *fakeClient) MoveFiles(ctx context.Context, srcs []string, dst string, mode freeboxTypes.FileMoveMode) (freeboxTypes.FileSystemTask, error) {
	if f.moveFilesFn != nil {
		return f.moveFilesFn(ctx, srcs, dst, mode)
	}
	panic("MoveFiles not expected")
}
func (f *fakeClient) GetFileSystemTask(ctx context.Context, id int64) (freeboxTypes.FileSystemTask, error) {
	if f.getFileSystemTaskFn != nil {
		return f.getFileSystemTaskFn(ctx, id)
	}
	panic("GetFileSystemTask not expected")
}
func (f *fakeClient) RemoveFiles(ctx context.Context, paths []string) (freeboxTypes.FileSystemTask, error) {
	if f.removeFilesFn != nil {
		return f.removeFilesFn(ctx, paths)
	}
	return freeboxTypes.FileSystemTask{}, nil
}
func (f *fakeClient) ResizeVirtualDisk(ctx context.Context, p freeboxTypes.VirtualDisksResizePayload) (int64, error) {
	if f.resizeVirtualDiskFn != nil {
		return f.resizeVirtualDiskFn(ctx, p)
	}
	panic("ResizeVirtualDisk not expected")
}
func (f *fakeClient) GetVirtualDiskTask(ctx context.Context, id int64) (freeboxTypes.VirtualMachineDiskTask, error) {
	if f.getVirtualDiskTaskFn != nil {
		return f.getVirtualDiskTaskFn(ctx, id)
	}
	panic("GetVirtualDiskTask not expected")
}

// Unused interface methods — panic on unexpected calls.
func (f *fakeClient) WithAppID(string) freeboxclient.Client { panic("not implemented") }
func (f *fakeClient) WithPrivateToken(freeboxTypes.PrivateToken) freeboxclient.Client {
	panic("not implemented")
}
func (f *fakeClient) WithHTTPClient(freeboxclient.HTTPClient) freeboxclient.Client {
	panic("not implemented")
}
func (f *fakeClient) APIVersion(context.Context) (freeboxTypes.APIVersion, error) {
	panic("not implemented")
}
func (f *fakeClient) Authorize(context.Context, freeboxTypes.AuthorizationRequest) (freeboxTypes.PrivateToken, error) {
	panic("not implemented")
}
func (f *fakeClient) Login(context.Context) (freeboxTypes.Permissions, error) {
	panic("not implemented")
}
func (f *fakeClient) Logout(context.Context) error { panic("not implemented") }
func (f *fakeClient) ListPortForwardingRules(context.Context) ([]freeboxTypes.PortForwardingRule, error) {
	panic("not implemented")
}
func (f *fakeClient) GetPortForwardingRule(ctx context.Context, identifier int64) (freeboxTypes.PortForwardingRule, error) {
	panic("not implemented")
}
func (f *fakeClient) CreatePortForwardingRule(ctx context.Context, payload freeboxTypes.PortForwardingRulePayload) (freeboxTypes.PortForwardingRule, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdatePortForwardingRule(ctx context.Context, identifier int64, payload freeboxTypes.PortForwardingRulePayload) (freeboxTypes.PortForwardingRule, error) {
	panic("not implemented")
}
func (f *fakeClient) DeletePortForwardingRule(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) ListDHCPStaticLease(context.Context) ([]freeboxTypes.DHCPStaticLeaseInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) GetDHCPStaticLease(ctx context.Context, identifier string) (freeboxTypes.DHCPStaticLeaseInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateDHCPStaticLease(ctx context.Context, identifier string, payload freeboxTypes.DHCPStaticLeasePayload) (freeboxTypes.LanInterfaceHost, error) {
	panic("not implemented")
}
func (f *fakeClient) CreateDHCPStaticLease(ctx context.Context, payload freeboxTypes.DHCPStaticLeasePayload) (freeboxTypes.LanInterfaceHost, error) {
	panic("not implemented")
}
func (f *fakeClient) DeleteDHCPStaticLease(ctx context.Context, identifier string) error {
	panic("not implemented")
}
func (f *fakeClient) GetLanConfig(ctx context.Context) (freeboxTypes.LanConfig, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateLanConfig(ctx context.Context, payload freeboxTypes.LanConfig) (freeboxTypes.LanConfig, error) {
	panic("not implemented")
}
func (f *fakeClient) ListLanInterfaceInfo(context.Context) ([]freeboxTypes.LanInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) GetLanInterface(ctx context.Context, name string) ([]freeboxTypes.LanInterfaceHost, error) {
	if f.getLanInterfaceFn != nil {
		return f.getLanInterfaceFn(ctx, name)
	}
	panic("GetLanInterface not expected")
}
func (f *fakeClient) GetLanInterfaceHost(ctx context.Context, interfaceName, identifier string) (freeboxTypes.LanInterfaceHost, error) {
	panic("not implemented")
}
func (f *fakeClient) GetVirtualMachineInfo(context.Context) (freeboxTypes.VirtualMachinesInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) GetVirtualMachineDistributions(context.Context) ([]freeboxTypes.VirtualMachineDistribution, error) {
	panic("not implemented")
}
func (f *fakeClient) ListVirtualMachines(context.Context) ([]freeboxTypes.VirtualMachine, error) {
	panic("not implemented")
}
func (f *fakeClient) CreateVirtualMachine(ctx context.Context, payload freeboxTypes.VirtualMachinePayload) (freeboxTypes.VirtualMachine, error) {
	panic("not implemented")
}
func (f *fakeClient) GetVirtualMachine(ctx context.Context, identifier int64) (freeboxTypes.VirtualMachine, error) {
	if f.getVirtualMachineFn != nil {
		return f.getVirtualMachineFn(ctx, identifier)
	}
	panic("GetVirtualMachine not expected")
}
func (f *fakeClient) UpdateVirtualMachine(ctx context.Context, identifier int64, payload freeboxTypes.VirtualMachinePayload) (freeboxTypes.VirtualMachine, error) {
	panic("not implemented")
}
func (f *fakeClient) DeleteVirtualMachine(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) StartVirtualMachine(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) KillVirtualMachine(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) StopVirtualMachine(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) GetVirtualDiskInfo(ctx context.Context, path string) (freeboxTypes.VirtualDiskInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) CreateVirtualDisk(ctx context.Context, payload freeboxTypes.VirtualDisksCreatePayload) (int64, error) {
	panic("not implemented")
}
func (f *fakeClient) DeleteVirtualDiskTask(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) ListenEvents(ctx context.Context, events []freeboxTypes.EventDescription) (chan freeboxTypes.Event, error) {
	panic("not implemented")
}
func (f *fakeClient) GetFileInfo(ctx context.Context, path string) (freeboxTypes.FileInfo, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateFileSystemTask(ctx context.Context, identifier int64, payload freeboxTypes.FileSytemTaskUpdate) (freeboxTypes.FileSystemTask, error) {
	panic("not implemented")
}
func (f *fakeClient) ListFileSystemTasks(ctx context.Context) ([]freeboxTypes.FileSystemTask, error) {
	panic("not implemented")
}
func (f *fakeClient) DeleteFileSystemTask(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) CreateDirectory(ctx context.Context, parent, name string) (string, error) {
	panic("not implemented")
}
func (f *fakeClient) AddHashFileTask(ctx context.Context, payload freeboxTypes.HashPayload) (freeboxTypes.FileSystemTask, error) {
	panic("not implemented")
}
func (f *fakeClient) GetHashResult(ctx context.Context, identifier int64) (string, error) {
	panic("not implemented")
}
func (f *fakeClient) GetFile(ctx context.Context, path string) (freeboxTypes.File, error) {
	panic("not implemented")
}
func (f *fakeClient) EraseDownloadTask(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) UpdateDownloadTask(ctx context.Context, identifier int64, payload freeboxTypes.DownloadTaskUpdate) error {
	panic("not implemented")
}
func (f *fakeClient) FileUploadStart(ctx context.Context, input freeboxTypes.FileUploadStartActionInput) (io.WriteCloser, int64, error) {
	panic("not implemented")
}
func (f *fakeClient) GetUploadTask(ctx context.Context, identifier int64) (freeboxTypes.UploadTask, error) {
	panic("not implemented")
}
func (f *fakeClient) ListUploadTasks(ctx context.Context) ([]freeboxTypes.UploadTask, error) {
	panic("not implemented")
}
func (f *fakeClient) CancelUploadTask(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) DeleteUploadTask(ctx context.Context, identifier int64) error {
	panic("not implemented")
}
func (f *fakeClient) CleanUploadTasks(ctx context.Context) error { panic("not implemented") }
func (f *fakeClient) CreateVPNUser(ctx context.Context, payload freeboxTypes.VPNUserPayload) (freeboxTypes.VPNUser, error) {
	panic("not implemented")
}
func (f *fakeClient) DeleteVPNUser(ctx context.Context, identifier string) error {
	panic("not implemented")
}
func (f *fakeClient) GetDownloadConfiguration(ctx context.Context) (freeboxTypes.DownloadConfiguration, error) {
	panic("not implemented")
}
func (f *fakeClient) GetOpenVPNServerConfig(ctx context.Context) (freeboxTypes.OpenVPNServerConfig, error) {
	panic("not implemented")
}
func (f *fakeClient) GetSystemInfo(ctx context.Context) (freeboxTypes.SystemConfig, error) {
	panic("not implemented")
}
func (f *fakeClient) GetVPNUser(ctx context.Context, identifier string) (freeboxTypes.VPNUser, error) {
	panic("not implemented")
}
func (f *fakeClient) GetVPNUserClientConfig(ctx context.Context, identifier string) (string, error) {
	panic("not implemented")
}
func (f *fakeClient) ListVPNUsers(ctx context.Context) ([]freeboxTypes.VPNUser, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateOpenVPNServerConfig(ctx context.Context, payload freeboxTypes.OpenVPNServerConfig) (freeboxTypes.OpenVPNServerConfig, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateVPNUser(ctx context.Context, login string, payload freeboxTypes.VPNUserPayload) (freeboxTypes.VPNUser, error) {
	panic("not implemented")
}
func (f *fakeClient) UpdateDownloadConfiguration(ctx context.Context, payload freeboxTypes.DownloadConfiguration) (freeboxTypes.DownloadConfiguration, error) {
	panic("not implemented")
}

// newMachineForPhaseTest creates a FreeboxMachine with the given name in the default namespace.
func newMachineForPhaseTest(name string, spec infrastructurev1alpha1.FreeboxMachineSpec) *infrastructurev1alpha1.FreeboxMachine {
	return &infrastructurev1alpha1.FreeboxMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: spec,
	}
}

var _ = Describe("FreeboxMachine phase transitions", func() {
	const (
		downloadDir   = "/mnt/downloads"
		vmStoragePath = "/mnt/VMs"
		imageURL      = "https://example.com/images/nocloud.raw.xz"
		imageName     = "nocloud.raw.xz"
		downloadPath  = "/mnt/downloads/nocloud.raw.xz"
		extractedBase = "nocloud.raw"
	)

	testCtx := context.Background()

	newReconciler := func(fc *fakeClient) *FreeboxMachineReconciler {
		return &FreeboxMachineReconciler{
			Client:             k8sClient,
			Scheme:             k8sClient.Scheme(),
			FreeboxClient:      fc,
			FreeboxDownloadDir: downloadDir,
			VMStoragePath:      vmStoragePath,
		}
	}

	Describe("TestPhaseDownload", func() {
		const resourceName = "phase-download-test"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "test-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      imageURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("reconcile with empty phase starts download and sets Phase=download", func() {
			fc := &fakeClient{
				listDownloadTasksFn: func(ctx context.Context) ([]freeboxTypes.DownloadTask, error) {
					return nil, nil // No existing tasks
				},
				addDownloadTaskFn: func(ctx context.Context, req freeboxTypes.DownloadRequest) (int64, error) {
					return 42, nil
				},
			}
			r := newReconciler(fc)
			result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(phaseDownload))
			Expect(updated.Status.TaskID).To(Equal(int64(42)))
		})
	})

	Describe("TestPhaseExtract", func() {
		const resourceName = "phase-extract-test"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "my-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      imageURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
			// Simulate download just completed: set phase=download, task done -> will transition to extract
			machine.Status.Phase = phaseDownload
			machine.Status.TaskID = 99
			Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("when download task is done, transitions to extract phase with taskID=0", func() {
			fc := &fakeClient{
				getDownloadTaskFn: func(ctx context.Context, id int64) (freeboxTypes.DownloadTask, error) {
					Expect(id).To(Equal(int64(99)))
					return freeboxTypes.DownloadTask{Status: freeboxTypes.DownloadTaskStatusDone}, nil
				},
			}
			r := newReconciler(fc)
			result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(phaseExtract))
			Expect(updated.Status.TaskID).To(Equal(int64(0)))
		})
	})

	Describe("TestPhaseCopy", func() {
		const resourceName = "phase-copy-test"
		// Use an uncompressed image so the copy path is exercised
		const uncompressedURL = "https://example.com/images/nocloud.raw"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "my-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      uncompressedURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
			machine.Status.Phase = phaseDownload
			machine.Status.TaskID = 77
			Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("when download task done for uncompressed image, transitions to copy phase", func() {
			fc := &fakeClient{
				getDownloadTaskFn: func(ctx context.Context, id int64) (freeboxTypes.DownloadTask, error) {
					return freeboxTypes.DownloadTask{Status: freeboxTypes.DownloadTaskStatusDone}, nil
				},
			}
			r := newReconciler(fc)
			result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(phaseCopy))
			Expect(updated.Status.TaskID).To(Equal(int64(0)))
		})
	})

	Describe("TestPhaseRename", func() {
		const resourceName = "phase-rename-test"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "my-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      imageURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
			// Simulate extract done -> rename pending
			machine.Status.Phase = phaseRename
			machine.Status.TaskID = 0
			machine.Status.RenameSrc = vmStoragePath + "/" + extractedBase
			machine.Status.RenameDst = vmStoragePath + "/my-vm.raw"
			Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("when rename task started and done, transitions to resize phase", func() {
			callCount := 0
			fc := &fakeClient{
				moveFilesFn: func(ctx context.Context, srcs []string, dst string, mode freeboxTypes.FileMoveMode) (freeboxTypes.FileSystemTask, error) {
					Expect(srcs).To(ConsistOf(vmStoragePath + "/" + extractedBase))
					Expect(dst).To(Equal(vmStoragePath + "/my-vm.raw"))
					callCount++
					return freeboxTypes.FileSystemTask{ID: 55}, nil
				},
			}
			r := newReconciler(fc)

			// First reconcile: starts the rename task
			result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())
			Expect(callCount).To(Equal(1))

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.TaskID).To(Equal(int64(55)))

			// Second reconcile: task is done → transition to resize
			fc.moveFilesFn = nil
			fc.getFileSystemTaskFn = func(ctx context.Context, id int64) (freeboxTypes.FileSystemTask, error) {
				Expect(id).To(Equal(int64(55)))
				return freeboxTypes.FileSystemTask{State: taskStateDone}, nil
			}
			result, err = r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())

			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(phaseResize))
			Expect(updated.Status.TaskID).To(Equal(int64(0)))
			Expect(updated.Status.RenameSrc).To(BeEmpty())
			Expect(updated.Status.RenameDst).To(BeEmpty())
		})
	})

	Describe("TestPhaseResize", func() {
		const resourceName = "phase-resize-test"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "my-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 20 * 1024 * 1024 * 1024,
				ImageURL:      imageURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
			machine.Status.Phase = phaseResize
			machine.Status.TaskID = 0
			Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("when resize task started and done, sets ImageReady condition (Phase stays resize until fully provisioned)", func() {
			callCount := 0
			fc := &fakeClient{
				resizeVirtualDiskFn: func(ctx context.Context, p freeboxTypes.VirtualDisksResizePayload) (int64, error) {
					callCount++
					return 88, nil
				},
			}
			r := newReconciler(fc)

			// First reconcile: starts resize task
			result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())
			Expect(callCount).To(Equal(1))

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			Expect(updated.Status.TaskID).To(Equal(int64(88)))

			// Second reconcile: task done → ImageReady condition set, Phase stays "resize"
			fc.resizeVirtualDiskFn = nil
			fc.getVirtualDiskTaskFn = func(ctx context.Context, id int64) (freeboxTypes.VirtualMachineDiskTask, error) {
				Expect(id).To(Equal(int64(88)))
				return freeboxTypes.VirtualMachineDiskTask{Done: true, Error: false}, nil
			}
			// The reconciler proceeds to VM creation after resize; since there is no CAPI
			// Machine owner it returns early. We verify ImageReady is set but Phase is NOT
			// prematurely "done" — that would break the IP-polling requeue loop.
			fc.listDownloadTasksFn = nil // not called in this path
			// May return error or requeue — the VM creation path requires owner Machine; that's fine.
			_, _ = r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})

			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())
			// Phase must remain "resize" until VM creation + IP assignment are complete.
			Expect(updated.Status.Phase).To(Equal(phaseResize))

			var imageReadyCond *metav1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == ConditionImageReady {
					imageReadyCond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(imageReadyCond).NotTo(BeNil())
			Expect(imageReadyCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Describe("TestPhaseError", func() {
		const resourceName = "phase-error-test"
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			machine := newMachineForPhaseTest(resourceName, infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          "my-vm",
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      imageURL,
			})
			Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
			machine.Status.Phase = phaseDownload
			machine.Status.TaskID = 111
			Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
		})

		AfterEach(func() {
			machine := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, nn, machine)
			_ = k8sClient.Delete(testCtx, machine)
		})

		It("when download task fails, sets ProvisioningFailed condition and returns error", func() {
			fc := &fakeClient{
				getDownloadTaskFn: func(ctx context.Context, id int64) (freeboxTypes.DownloadTask, error) {
					return freeboxTypes.DownloadTask{Status: freeboxTypes.DownloadTaskStatusError}, nil
				},
			}
			r := newReconciler(fc)
			_, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("download failed"))

			updated := &infrastructurev1alpha1.FreeboxMachine{}
			Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())

			var readyCond *metav1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == ReadyCondition {
					readyCond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Reason).To(Equal("ProvisioningFailed"))
		})
	})
})

// fakeClusterCache is a minimal ClusterCache implementation for unit tests.
// GetClient returns the configured client (or error) to simulate various
// workload cluster states.
type fakeClusterCache struct {
	getClientErr   error
	workloadClient client.Client
}

func (f *fakeClusterCache) GetClient(_ context.Context, _ client.ObjectKey) (client.Client, error) {
	if f.getClientErr != nil {
		return nil, f.getClientErr
	}
	return f.workloadClient, nil
}
func (f *fakeClusterCache) GetReader(_ context.Context, _ client.ObjectKey) (client.Reader, error) {
	panic("not implemented")
}
func (f *fakeClusterCache) GetUncachedClient(_ context.Context, _ client.ObjectKey) (client.Client, error) {
	panic("not implemented")
}
func (f *fakeClusterCache) GetRESTConfig(_ context.Context, _ client.ObjectKey) (*rest.Config, error) {
	panic("not implemented")
}
func (f *fakeClusterCache) GetClientCertificatePrivateKey(_ context.Context, _ client.ObjectKey) (*rsa.PrivateKey, error) {
	panic("not implemented")
}
func (f *fakeClusterCache) Watch(_ context.Context, _ client.ObjectKey, _ clustercache.Watcher) error {
	panic("not implemented")
}
func (f *fakeClusterCache) GetHealthCheckingState(_ context.Context, _ client.ObjectKey) clustercache.HealthCheckingState {
	panic("not implemented")
}
func (f *fakeClusterCache) GetClusterSource(_ string, _ func(context.Context, client.Object) []ctrl.Request, _ ...clustercache.GetClusterSourceOption) source.Source {
	panic("not implemented")
}

var _ clustercache.ClusterCache = &fakeClusterCache{}

var _ = Describe("FreeboxMachine phaseVMCreated provisioning", func() {
	// Regression test for the Talos deadlock:
	// The controller must set status.initialization.provisioned=true (and spec.providerID,
	// status.addresses) as soon as the VM has an IP address, WITHOUT waiting for the
	// workload cluster to be reachable. This unblocks bootstrap providers (e.g. Talos)
	// that need Machine.status.addresses before the workload cluster is up.
	const resourceName = "phase-vmcreated-provisioning-test"
	const vmID = int64(42)
	const vmMac = "aa:bb:cc:dd:ee:ff"
	const vmIP = "192.168.1.100"

	testCtx := context.Background()
	nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

	BeforeEach(func() {
		// Create a minimal CAPI Cluster so that GetClusterFromMetadata succeeds.
		// The Cluster CRD requires spec to be non-null; provide a dummy
		// controlPlaneEndpoint to satisfy the requirement.
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneEndpoint: clusterv1.APIEndpoint{
					Host: "127.0.0.1",
					Port: 6443,
				},
			},
		}
		Expect(k8sClient.Create(testCtx, cluster)).To(Succeed())

		// Create FreeboxMachine in phaseVMCreated with a VMID already set,
		// labelled to its owning Cluster
		machine := &infrastructurev1alpha1.FreeboxMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: resourceName,
				},
			},
			Spec: infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          resourceName,
				VCPUs:         1,
				MemoryMB:      512,
				DiskSizeBytes: 10 * 1024 * 1024 * 1024,
				ImageURL:      "https://example.com/image.raw",
			},
		}
		Expect(k8sClient.Create(testCtx, machine)).To(Succeed())

		machine.Status.Phase = phaseVMCreated
		machine.Status.VMID = &[]int64{vmID}[0]
		Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
	})

	AfterEach(func() {
		machine := &infrastructurev1alpha1.FreeboxMachine{}
		_ = k8sClient.Get(testCtx, nn, machine)
		_ = k8sClient.Delete(testCtx, machine)

		cluster := &clusterv1.Cluster{}
		_ = k8sClient.Get(testCtx, nn, cluster)
		_ = k8sClient.Delete(testCtx, cluster)
	})

	It("sets provisioned=true and addresses even when the workload cluster is unreachable", func() {
		fc := &fakeClient{
			getVirtualMachineFn: func(_ context.Context, id int64) (freeboxTypes.VirtualMachine, error) {
				Expect(id).To(Equal(vmID))
				return freeboxTypes.VirtualMachine{ID: vmID, Mac: vmMac}, nil
			},
			getLanInterfaceFn: func(_ context.Context, name string) ([]freeboxTypes.LanInterfaceHost, error) {
				Expect(name).To(Equal("pub"))
				return []freeboxTypes.LanInterfaceHost{
					{
						L2Ident: freeboxTypes.L2Ident{ID: vmMac},
						L3Connectivities: []freeboxTypes.LanHostL3Connectivity{
							{Type: "ipv4", Address: vmIP},
						},
					},
				}, nil
			},
		}

		r := &FreeboxMachineReconciler{
			Client:        k8sClient,
			Scheme:        k8sClient.Scheme(),
			FreeboxClient: fc,
			// Simulate an unreachable workload cluster
			ClusterCache: &fakeClusterCache{getClientErr: fmt.Errorf("cluster not connected")},
		}

		_, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		updated := &infrastructurev1alpha1.FreeboxMachine{}
		Expect(k8sClient.Get(testCtx, nn, updated)).To(Succeed())

		// CAPI contract: provisioned must be true so Machine.status.addresses gets populated
		Expect(updated.Status.Initialization.Provisioned).NotTo(BeNil())
		Expect(*updated.Status.Initialization.Provisioned).To(BeTrue(),
			"provisioned must be true even when the workload cluster is unreachable")

		// addresses must be set so CAPI can propagate them to Machine.status.addresses
		Expect(updated.Status.Addresses).NotTo(BeEmpty(),
			"addresses must be set when the VM has an IP")
		Expect(updated.Status.Addresses[0].Address).To(Equal(vmIP))

		// providerID must be set on the spec (CAPI contract)
		Expect(updated.Spec.ProviderID).To(Equal(fmt.Sprintf("freebox://%d", vmID)),
			"spec.providerID must be set alongside provisioned=true")

		// Ready condition must be True
		var readyCond *metav1.Condition
		for i := range updated.Status.Conditions {
			if updated.Status.Conditions[i].Type == ReadyCondition {
				readyCond = &updated.Status.Conditions[i]
				break
			}
		}
		Expect(readyCond).NotTo(BeNil())
		Expect(readyCond.Status).To(Equal(metav1.ConditionTrue),
			"Ready condition must be True once provisioned")
	})
})

// newFakeWorkloadClient builds a fake client seeded with the given objects,
// using the same scheme as the main test environment (includes corev1).
func newFakeWorkloadClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(k8sClient.Scheme()).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()
}

var _ = Describe("reconcileNodeProviderID — Talos random node names", func() {
	// Regression test: Talos does NOT use CloudHostName as the Kubernetes node name;
	// instead it generates a random name like "talos-xxx-yyy". The provider must
	// therefore find the workload-cluster node by matching its InternalIP against
	// FreeboxMachine.Status.Addresses rather than looking it up by FreeboxMachine.Name.

	const (
		vmID            = int64(0) // Freebox API returns 0 for the first VM
		vmIP            = "192.168.1.185"
		machineNodeName = "talos-kx7-0ys" // Talos-style random hostname — NOT the machine name
	)

	testCtx := context.Background()

	setupResources := func(resourceName string) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneEndpoint: clusterv1.APIEndpoint{
					Host: "192.168.1.202",
					Port: 6443,
				},
			},
		}
		Expect(k8sClient.Create(testCtx, cluster)).To(Succeed())
		DeferCleanup(func() {
			c := &clusterv1.Cluster{}
			_ = k8sClient.Get(testCtx, types.NamespacedName{Name: resourceName, Namespace: "default"}, c)
			_ = k8sClient.Delete(testCtx, c)
		})

		expectedProviderID := fmt.Sprintf("freebox://%d", vmID)
		machine := &infrastructurev1alpha1.FreeboxMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: resourceName,
				},
			},
			Spec: infrastructurev1alpha1.FreeboxMachineSpec{
				Name:          resourceName,
				VCPUs:         2,
				MemoryMB:      2048,
				DiskSizeBytes: 20 * 1024 * 1024 * 1024,
				ImageURL:      "https://example.com/image.raw",
				ProviderID:    expectedProviderID,
			},
		}
		Expect(k8sClient.Create(testCtx, machine)).To(Succeed())
		DeferCleanup(func() {
			m := &infrastructurev1alpha1.FreeboxMachine{}
			_ = k8sClient.Get(testCtx, types.NamespacedName{Name: resourceName, Namespace: "default"}, m)
			_ = k8sClient.Delete(testCtx, m)
		})

		vmIDVal := vmID
		machine.Status.Phase = phaseDone
		machine.Status.VMID = &vmIDVal
		machine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: vmIP},
		}
		Expect(k8sClient.Status().Update(testCtx, machine)).To(Succeed())
	}

	It("patches node providerID when node name differs from machine name (Talos random hostname)", func() {
		resourceName := "talos-lookup-bug-test"
		setupResources(resourceName)
		expectedProviderID := fmt.Sprintf("freebox://%d", vmID)

		// The workload cluster has a node named "talos-kx7-0ys" (Talos random name),
		// NOT matching the FreeboxMachine name. The node's InternalIP matches the machine IP.
		// Previously the controller looked up the node by FreeboxMachine name and failed;
		// now it must find the node by IP address.
		talosNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: machineNodeName,
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: vmIP},
				},
			},
		}
		workloadClient := newFakeWorkloadClient(talosNode)

		r := &FreeboxMachineReconciler{
			Client:       k8sClient,
			Scheme:       k8sClient.Scheme(),
			ClusterCache: &fakeClusterCache{workloadClient: workloadClient},
		}

		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}
		result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		// No requeue: node found by IP and patched successfully
		Expect(result.RequeueAfter).To(BeZero(),
			"no requeue expected once node is found by IP and patched")

		// Verify the node was patched with the correct providerID
		patchedNode := &corev1.Node{}
		Expect(workloadClient.Get(testCtx, client.ObjectKey{Name: machineNodeName}, patchedNode)).To(Succeed())
		Expect(patchedNode.Spec.ProviderID).To(Equal(expectedProviderID),
			"node providerID must be set even when the node name differs from the machine name")
	})

	It("patches node providerID when node is found by IP (expected behaviour after fix)", func() {
		resourceName := "talos-lookup-fix-test"
		setupResources(resourceName)
		expectedProviderID := fmt.Sprintf("freebox://%d", vmID)

		// Same scenario — after the fix, the node must be found by IP and patched.
		talosNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: machineNodeName,
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: vmIP},
				},
			},
		}
		workloadClient := newFakeWorkloadClient(talosNode)

		r := &FreeboxMachineReconciler{
			Client:       k8sClient,
			Scheme:       k8sClient.Scheme(),
			ClusterCache: &fakeClusterCache{workloadClient: workloadClient},
		}

		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}
		result, err := r.Reconcile(testCtx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		// After the fix: no requeue needed — the node was found by IP and patched
		Expect(result.RequeueAfter).To(BeZero(),
			"no requeue expected once node is found by IP and patched")

		// Verify the node WAS patched
		patchedNode := &corev1.Node{}
		Expect(workloadClient.Get(testCtx, client.ObjectKey{Name: machineNodeName}, patchedNode)).To(Succeed())
		Expect(patchedNode.Spec.ProviderID).To(Equal(expectedProviderID),
			"node providerID must be set to freebox://0 after fix")
	})
})
