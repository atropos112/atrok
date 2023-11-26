package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *AppBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	l := log.FromContext(ctx)

	// Get app bundle
	ab := &atroxyzv1alpha1.AppBundle{}
	if err := r.Get(ctx, req.NamespacedName, ab); err != nil {
		l.Error(err, "Unable to fetch app bundle, it was probably deleted, if not its a problem.")
		return ctrl.Result{}, err
	}

	// LOCK the resource
	mu := getMutex("app_bundle", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// Reconcile only if the observed generation is not the same as the current generation or
	// if the app bundle base has not been reconciled yet after it was updated
	if ab.Status.ObservedGeneration != ab.ObjectMeta.Generation {
		ab.Status.ObservedGeneration = ab.ObjectMeta.Generation
	} else if !ab.Status.AppBundleBaseReconciled {
		ab.Status.AppBundleBaseReconciled = true
	} else {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, ab); err != nil {
		l.Error(err, "Unable to update app bundle status.")
		return ctrl.Result{}, err
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

	return ctrl.Result{}, nil
}
