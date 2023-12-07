package controller

import (
	"context"
	"time"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	rxhash "github.com/rxwycdh/rxhash"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=atro.xyz,resources=appbundles/finalizers,verbs=update
var last_reconcilliation_ab map[string]int64 = make(map[string]int64)

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *AppBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	l := log.FromContext(ctx)

	// LOCK the resource
	mu := getMutex("app_bundle", req.Name, req.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// Get app bundle
	ab := &atroxyzv1alpha1.AppBundle{}
	if err := r.Get(ctx, req.NamespacedName, ab); err != nil {
		l.Error(err, "Unable to fetch app bundle, it was probably deleted, if not its a problem.")
		return ctrl.Result{}, err
	}

	// Reconcile only if the observed generation is not the same as the current generation or
	// if the app bundle base has not been reconciled yet after it was updated
	ab_hash, err := rxhash.HashStruct(ab.Spec)
	if err != nil {
		l.Error(err, "Unable to hash app bundle.")
		return ctrl.Result{}, err
	}

	if recon_time, ok := last_reconcilliation_ab[ab.Name]; ok && (time.Now().Unix()-recon_time < 60) && ab.Status.HashedSpec == ab_hash {
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	// We now update the last reconcilliation time and hash and reconcile
	last_reconcilliation_ab[ab.Name] = time.Now().Unix()
	if ab.Status.HashedSpec != ab_hash {
		ab.Status.HashedSpec = ab_hash
		if err := r.Status().Update(ctx, ab); err != nil {
			l.Error(err, "Unable to update app bundle status.")
			return ctrl.Result{}, err
		}
	}

	// Resolve app bundle base
	if ab.Spec.Base != nil {
		abb := &atroxyzv1alpha1.AppBundleBase{}
		if err := r.Get(ctx, client.ObjectKey{Name: *ab.Spec.Base}, abb); err != nil {
			l.Error(err, "Unable to fetch app bundle base, it was probably deleted, if not its a problem.")
			return ctrl.Result{}, err
		}
		err := ResolveAppBundleBase(ctx, r, ab, abb)
		if err != nil {
			l.Error(err, "Unable to resolve app bundle bases.")
			return ctrl.Result{}, err
		}
	}

	if err := RunReconciles(ctx, r, req, ab,
		r.ReconcileVolumes,
		r.ReconcileService,
		r.ReconcileDeployment,
		r.ReconcileIngress,
	); err != nil {
		l.Error(err, "Unable to reconcile app bundle.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}
