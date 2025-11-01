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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	freeboxclient "github.com/nikolalohinski/free-go/client"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

// FreeboxClusterReconciler reconciles a FreeboxCluster object
type FreeboxClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	FreeboxClient freeboxclient.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=freeboxclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FreeboxClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Fetch the FreeboxCluster resource
	var cluster infrastructurev1alpha1.FreeboxCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Following YAGNI principle: Since we don't manage external cluster infrastructure,
	// the cluster is always provisioned. We just need to report that to CAPI.

	// Set initialization.provisioned to true
	if cluster.Status.Initialization.Provisioned == nil || !*cluster.Status.Initialization.Provisioned {
		cluster.Status.Initialization.Provisioned = ptr.To(true)

		// Set Ready condition to True
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "InfrastructureReady",
			Message: "Freebox cluster infrastructure is ready",
		})

		if err := r.Status().Update(ctx, &cluster); err != nil {
			logger.Error(err, "Failed to update FreeboxCluster status")
			return ctrl.Result{}, err
		}

		logger.Info("FreeboxCluster marked as provisioned")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreeboxClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.FreeboxCluster{}).
		Named("freeboxcluster").
		Complete(r)
}
