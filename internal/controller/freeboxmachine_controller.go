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

	// Get the image URL from spec
	if machine.Spec.ImageURL == "" {
		logger.Info("No ImageURL specified in spec, skipping")
		return ctrl.Result{}, nil
	}
	imageURL := machine.Spec.ImageURL

	// Extract image name from URL
	parts := strings.Split(imageURL, "/")
	imageName := parts[len(parts)-1]

	destDir := "/SSD/VMs"

	// If no task exists, create it
	if machine.Status.DownloadTaskID == 0 {
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
			Type:    "ImageReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Downloading",
			Message: fmt.Sprintf("Download started, task_id=%d", taskID),
		})

		_ = r.Status().Update(ctx, &machine)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Fetch current status of the download task
	task, err := r.FreeboxClient.GetDownloadTask(ctx, machine.Status.DownloadTaskID)
	if err != nil {
		logger.Error(err, "Failed to get download task status", "taskID", machine.Status.DownloadTaskID)
		return ctrl.Result{}, err
	}

	// Update ImageReady condition based on task status
	switch task.Status {
	case freeboxTypes.DownloadTaskStatusDone:
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    "ImageReady",
			Status:  metav1.ConditionTrue,
			Reason:  "DownloadComplete",
			Message: "Image download completed",
		})
	case freeboxTypes.DownloadTaskStatusError:
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    "ImageReady",
			Status:  metav1.ConditionFalse,
			Reason:  "DownloadFailed",
			Message: "Image download failed",
		})
	default:
		progress := 0
		if task.SizeBytes > 0 {
			progress = int(task.ReceivedBytes * 100 / task.SizeBytes)
		}
		meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
			Type:    "ImageReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Downloading",
			Message: fmt.Sprintf("Download in progress… %d%%", progress),
		})
	}

	_ = r.Status().Update(ctx, &machine)

	// Requeue to continue tracking progress until done
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxMachine{}).
		Named("freeboxmachine").
		Complete(r)
}
