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
	"fmt"
	"path"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	freeboxclient "github.com/nikolalohinski/free-go/client"
	freeboxTypes "github.com/nikolalohinski/free-go/types"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionImageReady    = "ImageReady"
	ConditionVMProvisioned = "VMProvisioned"
	ConditionReady         = "Ready"

	FreeboxMachineFinalizer = "freeboxmachine.infrastructure.cluster.x-k8s.io/finalizer"
)

// FreeboxMachineReconciler reconciles a FreeboxMachine object
type FreeboxMachineReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	FreeboxClient      freeboxclient.Client
	FreeboxDownloadDir string // Freebox download directory path from /api/v*/downloads/config/
	VMStoragePath      string // VM storage path from user_main_storage + "/VMs"
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FreeboxMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *FreeboxMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Fetch the FreeboxMachine resource
	var machine infrastructurev1alpha1.FreeboxMachine
	if err := r.Get(ctx, req.NamespacedName, &machine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Handle deletion ---
	if !machine.ObjectMeta.DeletionTimestamp.IsZero() {
		if containsString(machine.Finalizers, FreeboxMachineFinalizer) {
			logger.Info("Deleting VM because FreeboxMachine is being deleted")

			vmID := machine.Status.VMID
			if vmID != 0 {
				if err := r.FreeboxClient.DeleteVirtualMachine(ctx, vmID); err != nil {
					logger.Error(err, "Failed to delete VM")
					return ctrl.Result{}, err
				}
				logger.Info("VM deleted", "vmID", vmID)
			}

			// Delete associated disk files
			diskPath := machine.Status.DiskPath
			if diskPath != "" {
				filesToDelete := []string{
					diskPath,              // .raw file
					diskPath + ".efivars", // .raw.efivars file
				}

				// Start file deletion task
				deleteTask, err := r.FreeboxClient.RemoveFiles(ctx, filesToDelete)
				if err != nil {
					logger.Error(err, "Failed to start disk file deletion", "files", filesToDelete)
					return ctrl.Result{}, err
				}
				logger.Info("Disk file deletion started", "taskID", deleteTask.ID, "files", filesToDelete)

				// We don't wait for the deletion to complete since it's cleanup
				// The files will be deleted asynchronously
			}

			// Remove finalizer
			machine.Finalizers = removeString(machine.Finalizers, FreeboxMachineFinalizer)
			if err := r.Update(ctx, &machine); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// --- Ensure finalizer ---
	if !containsString(machine.Finalizers, FreeboxMachineFinalizer) {
		machine.Finalizers = append(machine.Finalizers, FreeboxMachineFinalizer)
		if err := r.Update(ctx, &machine); err != nil {
			return ctrl.Result{}, err
		}
	}

	imageURL := machine.Spec.ImageURL
	if imageURL == "" {
		logger.Info("No ImageURL specified, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Images are downloaded to FreeboxDownloadDir, then extracted/copied to VMStoragePath
	imageName := path.Base(imageURL)
	downloadPath := path.Join(r.FreeboxDownloadDir, imageName)

	// Determine the final image path in VM storage
	var finalImagePath string
	if isCompressedFile(imageName) {
		// For compressed files, the extracted file won't have the compression extension
		finalImagePath = path.Join(r.VMStoragePath, removeCompressionExtension(imageName))
	} else {
		finalImagePath = path.Join(r.VMStoragePath, imageName)
	}

	// Retrieve current phase
	phaseCond := meta.FindStatusCondition(machine.Status.Conditions, "ImagePhase")
	var phase string
	var taskID int64

	if phaseCond != nil {
		fmt.Sscanf(phaseCond.Message, "phase=%s task_id=%d", &phase, &taskID)
	}

	// -----------------------
	// 1. Start download
	// -----------------------
	if phase == "" {
		logger.Info("Starting image download", "url", imageURL, "dest", r.FreeboxDownloadDir)

		reqDownload := freeboxTypes.DownloadRequest{
			DownloadURLs:      []string{imageURL},
			DownloadDirectory: r.FreeboxDownloadDir,
			Filename:          imageName,
		}

		newTaskID, err := r.FreeboxClient.AddDownloadTask(ctx, reqDownload)
		if err != nil {
			logger.Error(err, "Failed to create download task")
			return ctrl.Result{}, err
		}

		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    "ImagePhase",
			Status:  metav1.ConditionFalse,
			Reason:  "Downloading",
			Message: fmt.Sprintf("phase=download task_id=%d", newTaskID),
		})
		_ = r.Status().Update(ctx, &machine)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// -----------------------
	// 2. Wait for download
	// -----------------------
	if phase == "download" {
		downloadTask, err := r.FreeboxClient.GetDownloadTask(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to get download task status")
			return ctrl.Result{}, err
		}

		switch downloadTask.Status {
		case freeboxTypes.DownloadTaskStatusDone:
			logger.Info("Download completed", "taskID", taskID)

			if isCompressedFile(imageName) {
				// Extract from download dir to VM storage
				meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
					Type:    "ImagePhase",
					Status:  metav1.ConditionFalse,
					Reason:  "Extracting",
					Message: fmt.Sprintf("phase=extract task_id=0 src=%s dst=%s", downloadPath, r.VMStoragePath),
				})
			} else {
				// Copy from download dir to VM storage
				meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
					Type:    "ImagePhase",
					Status:  metav1.ConditionFalse,
					Reason:  "Copying",
					Message: fmt.Sprintf("phase=copy task_id=0 src=%s dst=%s", downloadPath, finalImagePath),
				})
			}
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil

		case freeboxTypes.DownloadTaskStatusError:
			logger.Error(fmt.Errorf("download failed"), "Download failed")
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "DownloadFailed",
				Message: "Image download failed",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{}, fmt.Errorf("download failed")

		default:
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// -----------------------
	// 3. Extraction phase
	// -----------------------
	if phase == "extract" {
		fmt.Sscanf(phaseCond.Message, "phase=extract task_id=%d", &taskID)

		if taskID == 0 {
			fsPayload := freeboxTypes.ExtractFilePayload{
				Src: freeboxTypes.Base64Path(downloadPath),
				Dst: freeboxTypes.Base64Path(r.VMStoragePath),
			}

			fsTask, err := r.FreeboxClient.ExtractFile(ctx, fsPayload)
			if err != nil {
				logger.Error(err, "Failed to start extraction")
				return ctrl.Result{}, err
			}

			logger.Info("Extraction started", "taskID", fsTask.ID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Extracting",
				Message: fmt.Sprintf("phase=extract task_id=%d", fsTask.ID),
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		fsTask, err := r.FreeboxClient.GetFileSystemTask(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to get extraction task status")
			return ctrl.Result{}, err
		}

		if fsTask.State == "done" {
			logger.Info("Extraction completed", "taskID", taskID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Resizing",
				Message: "phase=resize task_id=0",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		} else if fsTask.State == "error" {
			logger.Error(fmt.Errorf("extraction failed"), "Extraction failed")
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "ExtractionFailed",
				Message: "Image extraction failed",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{}, fmt.Errorf("extraction failed")
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// -----------------------
	// 4. Copy phase (for non-compressed images)
	// -----------------------
	if phase == "copy" {
		fmt.Sscanf(phaseCond.Message, "phase=copy task_id=%d", &taskID)

		if taskID == 0 {
			// Copy file from download dir to VM storage
			fsTask, err := r.FreeboxClient.CopyFiles(ctx, []string{downloadPath}, path.Dir(finalImagePath), freeboxTypes.FileCopyModeOverwrite)
			if err != nil {
				logger.Error(err, "Failed to start copy")
				return ctrl.Result{}, err
			}

			logger.Info("Copy started", "taskID", fsTask.ID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Copying",
				Message: fmt.Sprintf("phase=copy task_id=%d", fsTask.ID),
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		fsTask, err := r.FreeboxClient.GetFileSystemTask(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to get copy task status")
			return ctrl.Result{}, err
		}

		if fsTask.State == "done" {
			logger.Info("Copy completed", "taskID", taskID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Resizing",
				Message: "phase=resize task_id=0",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		} else if fsTask.State == "error" {
			logger.Error(fmt.Errorf("copy failed"), "Copy failed")
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "CopyFailed",
				Message: "Image copy failed",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{}, fmt.Errorf("copy failed")
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// -----------------------
	// 5. Resize disk
	// -----------------------
	if phase == "resize" {
		fmt.Sscanf(phaseCond.Message, "phase=resize task_id=%d", &taskID)

		if taskID == 0 {
			resizePayload := freeboxTypes.VirtualDisksResizePayload{
				DiskPath:    freeboxTypes.Base64Path(finalImagePath),
				NewSize:     machine.Spec.DiskSizeBytes,
				ShrinkAllow: false,
			}

			newTaskID, err := r.FreeboxClient.ResizeVirtualDisk(ctx, resizePayload)
			if err != nil {
				logger.Error(err, "Failed to start disk resize")
				return ctrl.Result{}, err
			}

			logger.Info("Resize task started", "taskID", newTaskID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Resizing",
				Message: fmt.Sprintf("phase=resize task_id=%d", newTaskID),
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		resizeTask, err := r.FreeboxClient.GetVirtualDiskTask(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to get resize task status")
			return ctrl.Result{}, err
		}

		if resizeTask.Done {
			if resizeTask.Error {
				logger.Error(fmt.Errorf("resize failed"), "Disk resize failed")
				meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
					Type:    "ImagePhase",
					Status:  metav1.ConditionFalse,
					Reason:  "ResizeFailed",
					Message: "Disk resize failed",
				})
				_ = r.Status().Update(ctx, &machine)
				return ctrl.Result{}, fmt.Errorf("resize failed")
			}

			logger.Info("Disk resize completed", "taskID", taskID)

			// Image is now ready (downloaded, extracted/copied, and resized)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImageReady",
				Status:  metav1.ConditionTrue,
				Reason:  "ImageReady",
				Message: "Image downloaded, extracted, and resized",
			})
			_ = r.Status().Update(ctx, &machine)

			// -----------------------
			// 5. Create VM
			// -----------------------
			vmPayload := freeboxTypes.VirtualMachinePayload{
				Name:     machine.Name,
				DiskPath: freeboxTypes.Base64Path(finalImagePath),
				DiskType: freeboxTypes.RawDisk,
				Memory:   machine.Spec.MemoryMB, // in MB
				VCPUs:    machine.Spec.VCPUs,
				OS:       freeboxTypes.UnknownOS,
			}

			vm, err := r.FreeboxClient.CreateVirtualMachine(ctx, vmPayload)
			if err != nil {
				logger.Error(err, "Failed to create virtual machine")
				return ctrl.Result{}, err
			}

			logger.Info("VM created", "vmID", vm.ID)

			// Store VM ID and disk path in status for deletion later
			machine.Status.VMID = vm.ID
			machine.Status.DiskPath = finalImagePath

			// Set initialization.provisioned to true - this signals to CAPI that the machine is ready
			machine.Status.Initialization.Provisioned = ptr.To(true)

			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    ConditionReady,
				Status:  metav1.ConditionTrue,
				Reason:  "VMCreated",
				Message: "VM created successfully",
			})
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    ConditionReady,
				Status:  metav1.ConditionTrue,
				Reason:  "InfrastructureReady",
				Message: "Freebox machine infrastructure is ready",
			})
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionTrue,
				Reason:  "Completed",
				Message: "phase=done",
			})
			_ = r.Status().Update(ctx, &machine)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// Helper to check if a file is a known compressed format
func isCompressedFile(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".gz", ".xz", ".bz2", ".zip", ".tar":
		return true
	default:
		return false
	}
}

// stripCompressionSuffix removes the trailing compression extension
// e.g. "nocloud.raw.xz" -> "nocloud.raw"
func stripCompressionSuffix(name string) string {
	lower := strings.ToLower(name)
	compressedExts := []string{".xz", ".gz", ".bz2", ".zip", ".tar"}
	for _, ext := range compressedExts {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	// fallback: use path.Ext trimming once
	if ext := path.Ext(name); ext != "" {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

// removeCompressionExtension is an alias for stripCompressionSuffix
func removeCompressionExtension(name string) string {
	return stripCompressionSuffix(name)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxMachine{}).
		Named("freeboxmachine").
		Complete(r)
}
