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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

// FreeboxClusterReconciler reconciles a FreeboxCluster object
type FreeboxClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FreeboxCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *FreeboxClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := log.FromContext(ctx)

	// Fetch the FreeboxCluster instance
	freeboxCluster := &infrastructurev1alpha1.FreeboxCluster{}
	if err := r.Get(ctx, req.NamespacedName, freeboxCluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster if it exists (optional for minimal setup)
	cluster, err := util.GetOwnerCluster(ctx, r.Client, freeboxCluster.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to get owner cluster")
		// Continue without owner cluster for minimal setup
	}
	if cluster != nil {
		log = log.WithValues("Cluster", klog.KObj(cluster))
		ctx = ctrl.LoggerInto(ctx, log)
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(freeboxCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the FreeboxCluster object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, freeboxCluster); err != nil {
			log.Error(err, "Failed to patch FreeboxCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Check if already provisioned (idempotency)
	if freeboxCluster.Status.Initialization != nil &&
		freeboxCluster.Status.Initialization.Provisioned != nil &&
		*freeboxCluster.Status.Initialization.Provisioned {
		log.Info("FreeboxCluster already provisioned, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// For a minimal single VM setup, we just mark the infrastructure as ready
	log.Info("Marking FreeboxCluster as provisioned")

	// Set initialization.provisioned to true
	provisioned := true
	if freeboxCluster.Status.Initialization == nil {
		freeboxCluster.Status.Initialization = &infrastructurev1alpha1.FreeboxClusterInitializationStatus{}
	}
	freeboxCluster.Status.Initialization.Provisioned = &provisioned

	log.Info("Successfully marked FreeboxCluster as provisioned")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxCluster{}).
		Named("freeboxcluster").
		Complete(r)
}
