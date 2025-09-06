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
)

// FreeboxMachineReconciler reconciles a FreeboxMachine object
type FreeboxMachineReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	FreeboxClient freeboxclient.Client
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

	// Fetch the FreeboxMachine
	var machine infrastructurev1alpha1.FreeboxMachine
	if err := r.Get(ctx, req.NamespacedName, &machine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	imageURL := machine.Spec.ImageURL
	if imageURL == "" {
		logger.Info("No ImageURL specified in spec, skipping")
		return ctrl.Result{}, nil
	}

	destDir := "/SSD/VMs"
	imageName := path.Base(imageURL)

	// ---------------------
	// Download phase
	// ---------------------
	if machine.Status.DownloadTaskID == 0 {
		logger.Info("Starting download phase", "url", imageURL)

		reqDownload := freeboxTypes.DownloadRequest{
			DownloadURLs:      []string{imageURL},
			DownloadDirectory: destDir,
			Filename:          imageName,
		}

		taskID, err := r.FreeboxClient.AddDownloadTask(ctx, reqDownload)
		if err != nil {
			logger.Error(err, "Failed to create download task")
			return ctrl.Result{}, err
		}

		logger.Info("Download task created", "taskID", taskID)
		machine.Status.DownloadTaskID = taskID
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    ConditionImageReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Downloading",
			Message: fmt.Sprintf("Download started, task_id=%d", taskID),
		})
		_ = r.Status().Update(ctx, &machine)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Wait for download to finish
	downloadTask, err := r.FreeboxClient.GetDownloadTask(ctx, machine.Status.DownloadTaskID)
	if err != nil {
		logger.Error(err, "Failed to get download task status")
		return ctrl.Result{}, err
	}

	switch downloadTask.Status {
	case freeboxTypes.DownloadTaskStatusDone:
		logger.Info("Download completed", "taskID", machine.Status.DownloadTaskID)
	case freeboxTypes.DownloadTaskStatusError:
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    ConditionImageReady,
			Status:  metav1.ConditionFalse,
			Reason:  "DownloadFailed",
			Message: fmt.Sprintf("Download task failed: %s", downloadTask.Error),
		})
		_ = r.Status().Update(ctx, &machine)
		return ctrl.Result{}, fmt.Errorf("download failed: %s", downloadTask.Error)
	default:
		// still downloading
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// ---------------------
	// Extraction phase
	// ---------------------
	if machine.Status.ExtractionTaskID == 0 && isCompressedFile(imageName) {
		srcPath := path.Join(destDir, imageName)
		extractDest := destDir

		fsPayload := freeboxTypes.ExtractFilePayload{
			Src: freeboxTypes.Base64Path(srcPath),
			Dst: freeboxTypes.Base64Path(extractDest),
		}

		fsTask, err := r.FreeboxClient.ExtractFile(ctx, fsPayload)
		if err != nil {
			logger.Error(err, "Failed to start extraction")
			return ctrl.Result{}, err
		}

		logger.Info("Extraction task created", "taskID", fsTask.ID)
		machine.Status.ExtractionTaskID = fsTask.ID
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    ConditionImageReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Extracting",
			Message: "Image extraction started",
		})
		_ = r.Status().Update(ctx, &machine)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Wait for extraction to finish
	if machine.Status.ExtractionTaskID != 0 {
		fsTask, err := r.FreeboxClient.GetFileSystemTask(ctx, machine.Status.ExtractionTaskID)
		if err != nil {
			logger.Error(err, "Failed to get extraction task status")
			return ctrl.Result{}, err
		}

		switch fsTask.State {
		case "done":
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    ConditionImageReady,
				Status:  metav1.ConditionTrue,
				Reason:  "Completed",
				Message: "Image downloaded and extracted",
			})
			_ = r.Status().Update(ctx, &machine)
			logger.Info("Extraction completed", "taskID", fsTask.ID)
			return ctrl.Result{}, nil

		case "error":
			meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
				Type:    ConditionImageReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ExtractionFailed",
				Message: "Image extraction failed",
			})
			_ = r.Status().Update(ctx, &machine)
			return ctrl.Result{}, fmt.Errorf("extraction failed")
		default:
			// still extracting
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// If not compressed, we consider the image ready
	meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
		Type:    ConditionImageReady,
		Status:  metav1.ConditionTrue,
		Reason:  "Completed",
		Message: "Image downloaded (no extraction needed)",
	})
	_ = r.Status().Update(ctx, &machine)
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

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxMachine{}).
		Named("freeboxmachine").
		Complete(r)
}
