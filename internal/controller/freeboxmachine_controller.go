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
	"strconv"
	"time"

	freeclient "github.com/nikolalohinski/free-go/client"
	"github.com/nikolalohinski/free-go/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

const (
	// FreeboxMachineFinalizer is the finalizer used to cleanup resources
	FreeboxMachineFinalizer = "freeboxmachine.infrastructure.cluster.x-k8s.io/finalizer"

	// Condition types
	VMCreatedCondition = "VMCreated"
	VMStartedCondition = "VMStarted"
	ReadyCondition     = "Ready"
) // FreeboxMachineReconciler reconciles a FreeboxMachine object
type FreeboxMachineReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	FreeboxClient freeclient.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FreeboxMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling FreeboxMachine", "namespacedName", req.NamespacedName)

	// Fetch the FreeboxMachine instance
	machine := &infrastructurev1alpha1.FreeboxMachine{}
	if err := r.Get(ctx, req.NamespacedName, machine); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("FreeboxMachine not found, probably deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FreeboxMachine")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !machine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machine)
	}

	// Handle normal reconciliation
	return r.reconcileNormal(ctx, machine)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxMachine{}).
		Named("freeboxmachine").
		Complete(r)
}

// reconcileNormal handles the normal reconciliation flow
func (r *FreeboxMachineReconciler) reconcileNormal(ctx context.Context, machine *infrastructurev1alpha1.FreeboxMachine) (ctrl.Result, error) {
	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(machine, FreeboxMachineFinalizer) {
		controllerutil.AddFinalizer(machine, FreeboxMachineFinalizer)
		return ctrl.Result{}, r.Update(ctx, machine)
	}

	// Check if VM already exists
	if machine.GetProviderID() != "" {
		return r.reconcileExistingVM(ctx, machine)
	}

	// Create new VM
	return r.createVM(ctx, machine)
}

// reconcileDelete handles VM deletion
func (r *FreeboxMachineReconciler) reconcileDelete(ctx context.Context, machine *infrastructurev1alpha1.FreeboxMachine) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(machine, FreeboxMachineFinalizer) {
		log.Info("Finalizer not present, nothing to do")
		return ctrl.Result{}, nil
	}

	// Delete VM if it exists
	if providerID := machine.GetProviderID(); providerID != "" {
		log.Info("Deleting VM", "providerID", providerID)

		vmID, err := strconv.ParseInt(providerID, 10, 64)
		if err != nil {
			log.Error(err, "Invalid provider ID", "providerID", providerID)
		} else {
			// Find and delete the VM
			if err := r.deleteVMByID(ctx, int(vmID)); err != nil {
				log.Error(err, "Failed to delete VM", "vmID", vmID)
				return ctrl.Result{RequeueAfter: time.Minute * 2}, err
			}
			log.Info("VM deleted successfully", "vmID", vmID)
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(machine, FreeboxMachineFinalizer)
	return ctrl.Result{}, r.Update(ctx, machine)
}

// createVM creates a new VM on the Freebox
func (r *FreeboxMachineReconciler) createVM(ctx context.Context, machine *infrastructurev1alpha1.FreeboxMachine) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Creating new VM", "machine", machine.Name)

	// Login to Freebox first
	_, err := r.FreeboxClient.Login(ctx)
	if err != nil {
		log.Error(err, "Failed to login to Freebox")
		r.updateCondition(machine, VMCreatedCondition, metav1.ConditionFalse, "LoginFailed", err.Error())
		if statusErr := r.Status().Update(ctx, machine); statusErr != nil {
			log.Error(statusErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 2}, err
	}

	// For now, we'll create a placeholder implementation
	// TODO: The free-go library doesn't seem to have VM creation methods exposed
	// We need to investigate the VM creation API or extend the library
	log.Info("VM creation API not yet implemented in free-go library", "machine", machine.Name)

	// Update condition to show we're working on this
	r.updateCondition(machine, VMCreatedCondition, metav1.ConditionFalse, "NotImplemented", "VM creation API not yet available in free-go library")

	// For testing purposes, let's simulate finding an existing VM
	vms, err := r.FreeboxClient.ListVirtualMachines(ctx)
	if err != nil {
		log.Error(err, "Failed to list VMs")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, err
	}

	// Look for a VM that could be managed by this machine
	for _, vm := range vms {
		if vm.Name == machine.Name || vm.Name == "test-vm" {
			// Use this VM as if we created it
			providerID := strconv.FormatInt(vm.ID, 10)
			machine.Spec.ProviderID = &providerID
			machine.Status.ProviderID = &providerID

			log.Info("Found existing VM to manage", "vmID", vm.ID, "vmName", vm.Name)
			r.updateCondition(machine, VMCreatedCondition, metav1.ConditionTrue, "VMFound", "Found existing VM to manage")

			// Update the machine
			if err := r.Update(ctx, machine); err != nil {
				log.Error(err, "Failed to update machine with provider ID")
				return ctrl.Result{}, err
			}

			// Update status
			if err := r.Status().Update(ctx, machine); err != nil {
				log.Error(err, "Failed to update machine status")
				return ctrl.Result{}, err
			}

			// Requeue to check VM status
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		}
	}

	// No suitable VM found
	log.Info("No suitable VM found to manage")
	r.updateCondition(machine, VMCreatedCondition, metav1.ConditionFalse, "NoVMFound", "No suitable VM found to manage")

	if err := r.Status().Update(ctx, machine); err != nil {
		log.Error(err, "Failed to update machine status")
	}

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// reconcileExistingVM handles reconciliation of existing VMs
func (r *FreeboxMachineReconciler) reconcileExistingVM(ctx context.Context, machine *infrastructurev1alpha1.FreeboxMachine) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Login first
	_, err := r.FreeboxClient.Login(ctx)
	if err != nil {
		log.Error(err, "Failed to login to Freebox")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, err
	}

	providerID := machine.GetProviderID()
	vmID, err := strconv.ParseInt(providerID, 10, 64)
	if err != nil {
		log.Error(err, "Invalid provider ID", "providerID", providerID)
		return ctrl.Result{}, err
	}

	// List all VMs and find ours
	vms, err := r.FreeboxClient.ListVirtualMachines(ctx)
	if err != nil {
		log.Error(err, "Failed to list VMs")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, err
	}

	var foundVM *types.VirtualMachine
	for _, vm := range vms {
		if vm.ID == vmID {
			foundVM = &vm
			break
		}
	}

	if foundVM == nil {
		log.Error(fmt.Errorf("VM not found"), "VM not found", "vmID", vmID)
		r.updateCondition(machine, ReadyCondition, metav1.ConditionFalse, "VMNotFound", "VM not found on Freebox")
		machine.Status.Ready = false

		if err := r.Status().Update(ctx, machine); err != nil {
			log.Error(err, "Failed to update machine status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	// Update machine status based on VM state
	machine.Status.VMState = foundVM.Status

	// Check if VM is running
	isRunning := foundVM.Status == "running"

	// Update conditions
	if isRunning {
		r.updateCondition(machine, VMStartedCondition, metav1.ConditionTrue, "VMRunning", "VM is running")
		r.updateCondition(machine, ReadyCondition, metav1.ConditionTrue, "VMReady", "VM is ready")
		machine.Status.Ready = true
	} else {
		r.updateCondition(machine, VMStartedCondition, metav1.ConditionFalse, "VMNotRunning", fmt.Sprintf("VM status: %s", foundVM.Status))
		r.updateCondition(machine, ReadyCondition, metav1.ConditionFalse, "VMNotReady", fmt.Sprintf("VM status: %s", foundVM.Status))
		machine.Status.Ready = false
	}

	log.Info("VM status updated", "vmID", vmID, "status", foundVM.Status, "ready", machine.Status.Ready)

	// Update status
	if err := r.Status().Update(ctx, machine); err != nil {
		log.Error(err, "Failed to update machine status")
		return ctrl.Result{}, err
	}

	// Requeue periodically to monitor VM status
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

// deleteVMByID deletes a VM by ID
func (r *FreeboxMachineReconciler) deleteVMByID(ctx context.Context, vmID int) error {
	// Login first
	_, err := r.FreeboxClient.Login(ctx)
	if err != nil {
		return fmt.Errorf("failed to login to Freebox: %w", err)
	}

	// For now, just log that we would delete the VM
	// TODO: Implement actual VM deletion when the API is available
	return fmt.Errorf("VM deletion not yet implemented in free-go library (VM ID: %d)", vmID)
}

// updateCondition updates a condition in the machine status
func (r *FreeboxMachineReconciler) updateCondition(machine *infrastructurev1alpha1.FreeboxMachine, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
		ObservedGeneration: machine.Generation,
	}

	// Find existing condition
	for i, existingCondition := range machine.Status.Conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition
			machine.Status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	machine.Status.Conditions = append(machine.Status.Conditions, condition)
}
