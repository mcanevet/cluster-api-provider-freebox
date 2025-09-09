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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
	freebox "github.com/nikolalohinski/free-go/client"
)

// FreeboxMachineReconciler reconciles a FreeboxMachine object
type FreeboxMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// FreeboxClient provides access to Freebox API operations
	FreeboxClient freebox.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FreeboxMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *FreeboxMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := log.FromContext(ctx)

	// Fetch the FreeboxMachine instance
	freeboxMachine := &infrastructurev1alpha1.FreeboxMachine{}
	if err := r.Get(ctx, req.NamespacedName, freeboxMachine); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine (optional for minimal setup)
	machine, err := util.GetOwnerMachine(ctx, r.Client, freeboxMachine.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to get owner machine")
		// Continue without owner machine for minimal setup
	}
	if machine != nil {
		log = log.WithValues("Machine", klog.KObj(machine))
		ctx = ctrl.LoggerInto(ctx, log)
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(freeboxMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the FreeboxMachine object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, freeboxMachine); err != nil {
			log.Error(err, "Failed to patch FreeboxMachine")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Check if already provisioned (idempotency)
	if freeboxMachine.Status.Initialization != nil &&
		freeboxMachine.Status.Initialization.Provisioned != nil &&
		*freeboxMachine.Status.Initialization.Provisioned &&
		freeboxMachine.Spec.ProviderID != "" {
		log.Info("FreeboxMachine already provisioned, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Ensure we have a Freebox client
	if r.FreeboxClient == nil {
		return ctrl.Result{}, fmt.Errorf("FreeboxClient is not initialized")
	}

	// For minimal setup, simulate VM provisioning workflow
	log.Info("Provisioning FreeboxMachine", "imageURL", freeboxMachine.Spec.ImageURL)

	// Implement the VM provisioning steps from AGENTS.md:
	// 1. Download image, 2. Extract, 3. Resize, 4. Create VM, 5. Start VM

	// Step 1: Download image (first implementation)
	if err := r.downloadImage(ctx, freeboxMachine); err != nil {
		log.Error(err, "Failed to download VM image")
		return ctrl.Result{}, err
	} // Generate a unique provider ID (simulate VM creation)
	if freeboxMachine.Spec.ProviderID == "" {
		vmID := fmt.Sprintf("vm-%s", freeboxMachine.Name)
		freeboxMachine.Spec.ProviderID = fmt.Sprintf("freebox:///%s", vmID)
		log.Info("Generated provider ID", "providerID", freeboxMachine.Spec.ProviderID)
	}

	// Set initialization.provisioned to true
	provisioned := true
	if freeboxMachine.Status.Initialization == nil {
		freeboxMachine.Status.Initialization = &infrastructurev1alpha1.FreeboxMachineInitializationStatus{}
	}
	freeboxMachine.Status.Initialization.Provisioned = &provisioned

	log.Info("Successfully marked FreeboxMachine as provisioned", "providerID", freeboxMachine.Spec.ProviderID)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxMachine{}).
		Named("freeboxmachine").
		Complete(r)
}

// downloadImage implements step 1 of the VM provisioning workflow
func (r *FreeboxMachineReconciler) downloadImage(ctx context.Context, machine *infrastructurev1alpha1.FreeboxMachine) error {
	log := log.FromContext(ctx)

	// For now, we'll simulate the download and log the operation
	// TODO: Implement actual image download to Freebox storage
	log.Info("Downloading VM image",
		"imageURL", machine.Spec.ImageURL,
		"vmName", machine.Name,
		"diskSize", machine.Spec.DiskSize,
	)

	// Validate image URL format
	if machine.Spec.ImageURL == "" {
		return fmt.Errorf("imageURL cannot be empty")
	}

	// TODO: Add actual Freebox API calls for image download
	// This would involve:
	// 1. Check if image already exists in Freebox storage
	// 2. If not, initiate download from imageURL
	// 3. Monitor download progress
	// 4. Validate downloaded image

	log.Info("Image download completed (simulated)", "imageURL", machine.Spec.ImageURL)
	return nil
}
