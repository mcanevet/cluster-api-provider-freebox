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
	"regexp"
	"strings"
	"time"

	freeboxclient "github.com/nikolalohinski/free-go/client"
	freeboxTypes "github.com/nikolalohinski/free-go/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

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
			if vmID != nil {
				// Force stop (kill) the VM before deletion - Freebox API requires VMs to be stopped before deletion
				logger.Info("Force stopping VM before deletion", "vmID", *vmID)
				if err := r.FreeboxClient.KillVirtualMachine(ctx, *vmID); err != nil {
					logger.Error(err, "Failed to force stop VM (may already be stopped)")
					// Don't return error here - the VM might already be stopped
				}

				// Wait for VM to be fully stopped before attempting deletion
				logger.Info("Waiting for VM to stop", "vmID", *vmID)
				for i := 0; i < 30; i++ { // Wait up to 30 seconds
					vm, err := r.FreeboxClient.GetVirtualMachine(ctx, *vmID)
					if err != nil {
						logger.Error(err, "Failed to get VM status while waiting for stop")
						break
					}

					if vm.Status == "stopped" {
						logger.Info("VM is now stopped", "vmID", *vmID)
						break
					}

					logger.Info("VM not yet stopped, waiting...", "vmID", *vmID, "status", vm.Status, "attempt", i+1)
					time.Sleep(1 * time.Second)
				}

				// Now delete the VM
				if err := r.FreeboxClient.DeleteVirtualMachine(ctx, *vmID); err != nil {
					logger.Error(err, "Failed to delete VM")
					return ctrl.Result{}, err
				}
				logger.Info("VM deleted", "vmID", *vmID)
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

	// Determine the final image path in VM storage using VM name
	// The final image will be named after the VM (machine.Spec.Name) with the underlying disk extension
	underlyingName := imageName
	if isCompressedFile(imageName) {
		underlyingName = removeCompressionExtension(imageName)
	}
	ext := path.Ext(underlyingName)
	if ext == "" {
		ext = ".raw" // Default extension if none found
	}
	vmImageName := machine.Spec.Name + ext
	finalImagePath := path.Join(r.VMStoragePath, vmImageName)

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

			// After extraction, file has the underlying name (without compression suffix)
			// Need to rename to VM-named file
			extractedPath := path.Join(r.VMStoragePath, removeCompressionExtension(imageName))
			if extractedPath != finalImagePath {
				logger.Info("Starting rename after extraction", "from", extractedPath, "to", finalImagePath)
				meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
					Type:    "ImagePhase",
					Status:  metav1.ConditionFalse,
					Reason:  "Renaming",
					Message: fmt.Sprintf("phase=rename task_id=0 src=%s dst=%s", extractedPath, finalImagePath),
				})
				_ = r.Status().Update(ctx, &machine)
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

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
			// Copy file from download dir to VM storage directory
			// Note: CopyFiles can only specify directory destination, not filename
			// We'll copy to VM storage dir, keeping the original in downloads
			fsTask, err := r.FreeboxClient.CopyFiles(ctx, []string{downloadPath}, r.VMStoragePath, freeboxTypes.FileCopyModeOverwrite)
			if err != nil {
				logger.Error(err, "Failed to start copy to VM storage")
				return ctrl.Result{}, err
			}

			logger.Info("Copy started", "taskID", fsTask.ID, "from", downloadPath, "to", r.VMStoragePath)
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

			// After copy completes, we need to rename from source filename to VM name
			// The copied file has the source image name, we need to rename it to VM name
			copiedPath := path.Join(r.VMStoragePath, imageName)
			if copiedPath != finalImagePath {
				// Need to rename the copied file to the VM-named path
				meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
					Type:    "ImagePhase",
					Status:  metav1.ConditionFalse,
					Reason:  "Renaming",
					Message: fmt.Sprintf("phase=rename task_id=0 src=%s dst=%s", copiedPath, finalImagePath),
				})
				_ = r.Status().Update(ctx, &machine)
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

			// If names already match (shouldn't happen), proceed to resize
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
	// 5. Rename to VM name
	// -----------------------
	if phase == "rename" {
		var srcPath, dstPath string
		// Parse the message to extract task_id, src, and dst
		// Use regex to handle paths with spaces
		re := regexp.MustCompile(`task_id=(\d+) src=(.+) dst=(.+)`)
		matches := re.FindStringSubmatch(phaseCond.Message)
		if len(matches) == 4 {
			fmt.Sscanf(matches[1], "%d", &taskID)
			srcPath = matches[2]
			dstPath = matches[3]
		}

		if taskID == 0 {
			// Start the rename operation using MoveFiles
			mvTask, err := r.FreeboxClient.MoveFiles(ctx, []string{srcPath}, dstPath, freeboxTypes.FileMoveModeOverwrite)
			if err != nil {
				logger.Error(err, "Failed to start rename", "from", srcPath, "to", dstPath)
				return ctrl.Result{}, err
			}

			logger.Info("Rename task started", "taskID", mvTask.ID, "from", srcPath, "to", dstPath)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Renaming",
				Message: fmt.Sprintf("phase=rename task_id=%d src=%s dst=%s", mvTask.ID, srcPath, dstPath),
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		fsTask, err := r.FreeboxClient.GetFileSystemTask(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to get rename task status")
			return ctrl.Result{}, err
		}

		if fsTask.State == "done" {
			logger.Info("Rename completed", "taskID", taskID)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "Resizing",
				Message: "phase=resize task_id=0",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		} else if fsTask.State == "error" {
			logger.Error(fmt.Errorf("rename failed"), "Rename failed", "error", fsTask.Error)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImagePhase",
				Status:  metav1.ConditionFalse,
				Reason:  "RenameFailed",
				Message: fmt.Sprintf("Image rename failed: %s", fsTask.Error),
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{}, fmt.Errorf("rename failed: %s", fsTask.Error)
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// -----------------------
	// 6. Resize disk
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

			// Image is now ready (downloaded, extracted/copied, renamed, and resized)
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    "ImageReady",
				Status:  metav1.ConditionTrue,
				Reason:  "ImageReady",
				Message: "Image downloaded, extracted, renamed, and resized",
			})
			if err := r.Status().Update(ctx, &machine); err != nil {
				// Ignore conflict errors - another reconcile already updated the object
				if !errors.IsConflict(err) {
					logger.Error(err, "Failed to update status after resize")
					return ctrl.Result{}, err
				}
				logger.Info("Status update conflict, another reconcile already updated - continuing")
			} // -----------------------
			// 7. Create VM (or check IP if already created)
			// -----------------------
			// Check if VM already exists (in case status update succeeded but reconcile restarted)
			if machine.Status.VMID != nil {
				logger.Info("VM already created, checking for IP address", "vmID", *machine.Status.VMID)

				// VM exists, but we might still need to populate IP address
				// Check if addresses are already populated
				if len(machine.Status.Addresses) > 0 {
					// Addresses already set, nothing more to do
					logger.Info("VM already has IP addresses", "vmID", *machine.Status.VMID, "addresses", machine.Status.Addresses)
					return ctrl.Result{}, nil
				}

				// Try to get the VM to retrieve its MAC address
				vm, err := r.FreeboxClient.GetVirtualMachine(ctx, *machine.Status.VMID)
				if err != nil {
					logger.Error(err, "Failed to get VM details")
					return ctrl.Result{}, err
				}

				// Try to get IP address from LAN browser
				lanHosts, err := r.FreeboxClient.GetLanInterface(ctx, "pub")
				if err != nil {
					logger.Error(err, "Failed to query LAN browser")
					// Don't fail the reconciliation, just requeue to try again
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}

				logger.Info("Searching for VM in LAN browser", "vmID", *machine.Status.VMID, "vmMac", vm.Mac, "totalHosts", len(lanHosts))

				// Find the host with matching MAC address (case-insensitive comparison)
				foundHost := false
				vmMacLower := strings.ToLower(vm.Mac)
				for _, host := range lanHosts {
					hostMacLower := strings.ToLower(host.L2Ident.ID)
					if hostMacLower == vmMacLower {
						foundHost = true
						// Extract IPv4 addresses from L3Connectivities
						var addresses []clusterv1.MachineAddress
						for _, l3 := range host.L3Connectivities {
							if l3.Type == "ipv4" && l3.Address != "" {
								addresses = append(addresses, clusterv1.MachineAddress{
									Type:    clusterv1.MachineInternalIP,
									Address: l3.Address,
								})
							}
						}
						if len(addresses) > 0 {
							machine.Status.Addresses = addresses
							logger.Info("Found IP address for existing VM", "vmID", *machine.Status.VMID, "mac", vm.Mac, "addresses", addresses)
							if err := r.Status().Update(ctx, &machine); err != nil {
								logger.Error(err, "Failed to update FreeboxMachine status with addresses")
								return ctrl.Result{}, err
							}
							return ctrl.Result{}, nil
						} else {
							logger.Info("VM found in LAN browser but no IP address yet, will retry", "vmID", *machine.Status.VMID, "mac", vm.Mac)
							return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
						}
					}
				}

				if !foundHost {
					logger.Info("VM not yet visible in LAN browser, will retry", "vmID", *machine.Status.VMID, "mac", vm.Mac)
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}
			}

			// -----------------------
			// Get Machine owner and bootstrap data
			// -----------------------
			// Get the Machine that owns this FreeboxMachine
			ownerMachine, err := util.GetOwnerMachine(ctx, r.Client, machine.ObjectMeta)
			if err != nil {
				logger.Error(err, "Failed to get owner Machine")
				return ctrl.Result{}, err
			}
			if ownerMachine == nil {
				logger.Info("FreeboxMachine has no owner Machine yet, waiting")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}

			logger.Info("Found owner Machine", "machineName", ownerMachine.Name, "namespace", ownerMachine.Namespace)

			// Check if bootstrap data is ready
			if ownerMachine.Spec.Bootstrap.DataSecretName == nil {
				logger.Info("Bootstrap data secret not ready yet, waiting", "machineName", ownerMachine.Name)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}

			logger.Info("Bootstrap data secret is ready", "secretName", *ownerMachine.Spec.Bootstrap.DataSecretName)

			// Read the bootstrap data secret
			bootstrapSecret := &corev1.Secret{}
			secretKey := types.NamespacedName{
				Namespace: ownerMachine.Namespace,
				Name:      *ownerMachine.Spec.Bootstrap.DataSecretName,
			}
			if err := r.Get(ctx, secretKey, bootstrapSecret); err != nil {
				logger.Error(err, "Failed to get bootstrap data secret", "secretName", secretKey.Name)
				return ctrl.Result{}, err
			}

			// Extract bootstrap data from the secret
			// The bootstrap data is stored in the "value" key
			bootstrapData, ok := bootstrapSecret.Data["value"]
			if !ok {
				logger.Error(fmt.Errorf("bootstrap secret missing 'value' key"), "Invalid bootstrap secret", "secretName", secretKey.Name)
				return ctrl.Result{}, fmt.Errorf("bootstrap secret %s missing 'value' key", secretKey.Name)
			}

			logger.Info("Successfully retrieved bootstrap data", "secretName", secretKey.Name, "dataSize", len(bootstrapData))

			// Determine disk type based on the final image file extension
			diskType := freeboxTypes.RawDisk // Default to raw
			finalExt := strings.ToLower(path.Ext(finalImagePath))
			if finalExt == ".qcow2" {
				diskType = freeboxTypes.QCow2Disk
				logger.Info("Using qcow2 disk type", "imagePath", finalImagePath)
			} else {
				logger.Info("Using raw disk type", "imagePath", finalImagePath, "extension", finalExt)
			}

			vmPayload := freeboxTypes.VirtualMachinePayload{
				Name:              machine.Name,
				DiskPath:          freeboxTypes.Base64Path(finalImagePath),
				DiskType:          diskType,
				Memory:            machine.Spec.MemoryMB, // in MB
				VCPUs:             machine.Spec.VCPUs,
				OS:                freeboxTypes.UnknownOS,
				EnableCloudInit:   true,
				CloudInitUserData: string(bootstrapData),
			}

			vm, err := r.FreeboxClient.CreateVirtualMachine(ctx, vmPayload)
			if err != nil {
				logger.Error(err, "Failed to create virtual machine")
				return ctrl.Result{}, err
			}

			logger.Info("VM created successfully", "vmID", vm.ID, "name", vm.Name)

			// Set providerID in spec to match CAPI contract
			// Format: freebox://<vm-id>
			// Only update if not already set to avoid unnecessary API calls
			if machine.Spec.ProviderID == "" {
				providerID := fmt.Sprintf("freebox://%d", vm.ID)
				machine.Spec.ProviderID = providerID
				if err := r.Update(ctx, &machine); err != nil {
					logger.Error(err, "Failed to update FreeboxMachine spec with providerID")
					return ctrl.Result{}, err
				}
				logger.Info("Set providerID", "providerID", providerID)
			}

			// Store VM ID and disk path in status immediately after creation
			// This ensures we can clean up the VM even if subsequent operations fail
			machine.Status.VMID = &vm.ID
			machine.Status.DiskPath = finalImagePath

			// Start the VM
			if err := r.FreeboxClient.StartVirtualMachine(ctx, vm.ID); err != nil {
				logger.Error(err, "Failed to start virtual machine")
				return ctrl.Result{}, err
			}

			logger.Info("VM started", "vmID", vm.ID)

			// Try to get IP address from LAN browser
			// Query the LAN browser for hosts on the "pub" interface
			lanHosts, err := r.FreeboxClient.GetLanInterface(ctx, "pub")
			if err != nil {
				logger.Error(err, "Failed to query LAN browser")
				// Don't fail the reconciliation, just log and continue without addresses
			} else {
				logger.Info("Searching for VM in LAN browser", "vmID", vm.ID, "vmMac", vm.Mac, "totalHosts", len(lanHosts))
				// Find the host with matching MAC address (case-insensitive comparison)
				foundHost := false
				vmMacLower := strings.ToLower(vm.Mac)
				for _, host := range lanHosts {
					hostMacLower := strings.ToLower(host.L2Ident.ID)
					if hostMacLower == vmMacLower {
						foundHost = true
						// Extract IPv4 addresses from L3Connectivities
						var addresses []clusterv1.MachineAddress
						for _, l3 := range host.L3Connectivities {
							if l3.Type == "ipv4" && l3.Address != "" {
								addresses = append(addresses, clusterv1.MachineAddress{
									Type:    clusterv1.MachineInternalIP,
									Address: l3.Address,
								})
							}
						}
						if len(addresses) > 0 {
							machine.Status.Addresses = addresses
							logger.Info("Found IP address for VM", "vmID", vm.ID, "mac", vm.Mac, "addresses", addresses)
						} else {
							logger.Info("VM found in LAN browser but no IP address yet, will retry", "vmID", vm.ID, "mac", vm.Mac)
							// VM is in LAN browser but no IP yet - requeue to check again
							machine.Status.VMID = &vm.ID
							machine.Status.DiskPath = finalImagePath
							if err := r.Status().Update(ctx, &machine); err != nil {
								logger.Error(err, "Failed to update FreeboxMachine status")
								return ctrl.Result{}, err
							}
							return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
						}
						break
					}
				}
				if !foundHost {
					logger.Info("VM not yet visible in LAN browser, will retry", "vmID", vm.ID, "mac", vm.Mac)
					// VM not yet in LAN browser - requeue to check again
					machine.Status.VMID = &vm.ID
					machine.Status.DiskPath = finalImagePath
					if err := r.Status().Update(ctx, &machine); err != nil {
						logger.Error(err, "Failed to update FreeboxMachine status")
						return ctrl.Result{}, err
					}
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}
			}

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
			if err := r.Status().Update(ctx, &machine); err != nil {
				// Ignore conflict errors - another reconcile already updated the object
				if !errors.IsConflict(err) {
					logger.Error(err, "Failed to update status after VM creation")
					return ctrl.Result{}, err
				}
				logger.Info("Status update conflict, another reconcile already updated - continuing")
			}

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
