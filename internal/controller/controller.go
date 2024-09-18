package controller

import (
	"context"
	"time"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *AppBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// LOCK the resource
	mu := getMutex("appBundle", req.Name, req.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// Get app bundle
	ab := &atroxyzv1alpha1.AppBundle{}
	if err := r.Get(ctx, req.NamespacedName, ab); err != nil {
		return ctrl.Result{RequeueAfter: 120 * time.Second}, err
	}

	// Get (cached) state of the appbundle
	err := RegisterStateIfNotAlreadyRegistered(ab)
	stateAlreadyRegistered := false
	if err != nil {
		_, stateAlreadyRegistered = err.(StateAlreadyRegisteredError)
		if !stateAlreadyRegistered {
			return ctrl.Result{RequeueAfter: 120 * time.Second}, err
		}
	}

	stateNeedsUpdating, err := StateNeedsUpdating(ab, stateAlreadyRegistered)
	if err != nil {
		return ctrl.Result{RequeueAfter: 120 * time.Second}, err
	}

	if !stateNeedsUpdating {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Resolve app bundle base
	if ab.Spec.Base != nil {
		abb := &atroxyzv1alpha1.AppBundleBase{}
		if err := r.Get(ctx, client.ObjectKey{Name: *ab.Spec.Base}, abb); err != nil {
			return ctrl.Result{RequeueAfter: 120 * time.Second}, err
		}
		err := ResolveAppBundleBase(ctx, r, ab, abb)
		if err != nil {
			return ctrl.Result{RequeueAfter: 120 * time.Second}, err
		}
	}

	if err := RunReconciles(ctx, ab,
		r.ReconcileVolumes,
		// r.ReconcileService,
		// r.ReconcileDeployment,
		// r.ReconcileIngress,
		// r.ReconcileConfigMap,
		// r.ReconcileExternalSecret,
	); err != nil {
		// TODO: Given an error, we should consider running exponential backoff here.
		return ctrl.Result{RequeueAfter: 120 * time.Second}, err
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}
