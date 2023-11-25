package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func ResolveAppBundleBase(ctx context.Context, r *AppBundleReconciler, ab *atroxyzv1alpha1.AppBundle, abb *atroxyzv1alpha1.AppBundleBase) error {
	abSpec := &ab.Spec
	abbSpec := &abb.Spec

	// By hand merge, can do with reflection but then its not clear when to override, when to append etc.

	if abbSpec.Volumes != nil {
		if abSpec.Volumes == nil {
			abSpec.Volumes = append(abSpec.Volumes, abbSpec.Volumes...)
		} else {
			abSpec.Volumes = abbSpec.Volumes
		}
	}

	if abbSpec.Routes != nil {
		if abSpec.Routes == nil {
			abSpec.Routes = append(abSpec.Routes, abbSpec.Routes...)
		} else {
			abSpec.Routes = abbSpec.Routes
		}
	}

	if abbSpec.Resources != nil && abSpec.Resources == nil {
		abSpec.Resources = abbSpec.Resources
	}

	if abbSpec.Replicas != nil && abSpec.Replicas == nil {
		abSpec.Replicas = abbSpec.Replicas
	}

	// Special case, happy to fill in the blanks but not the whole things,
	// it makes no sense to inherit the whole thing, so it needs to exist in some capacity beforehand
	if abbSpec.Homepage != nil && abSpec.Homepage != nil {
		if abbSpec.Homepage.Name != nil && abSpec.Homepage.Name == nil {
			abSpec.Homepage.Name = abbSpec.Homepage.Name
		}

		if abbSpec.Homepage.Description != nil && abSpec.Homepage.Description == nil {
			abSpec.Homepage.Description = abbSpec.Homepage.Description
		}

		if abbSpec.Homepage.Group != nil && abSpec.Homepage.Group == nil {
			abSpec.Homepage.Group = abbSpec.Homepage.Group
		}

		if abbSpec.Homepage.Href != nil && abSpec.Homepage.Href == nil {
			abSpec.Homepage.Href = abbSpec.Homepage.Href
		}

		if abbSpec.Homepage.Icon != nil && abSpec.Homepage.Icon == nil {
			abSpec.Homepage.Icon = abbSpec.Homepage.Icon
		}
	}

	if abbSpec.Image != nil {
		if abSpec.Image == nil {
			abSpec.Image = &atroxyzv1alpha1.AppBundleImage{
				Repository: abbSpec.Image.Repository,
				Tag:        abbSpec.Image.Tag,
				PullPolicy: abbSpec.Image.PullPolicy,
			}
		} else {
			if abbSpec.Image.Repository != nil && abSpec.Image.Repository == nil {
				abSpec.Image.Repository = abbSpec.Image.Repository
			}
			if abbSpec.Image.Tag != nil && abSpec.Image.Tag == nil {
				abSpec.Image.Tag = abbSpec.Image.Tag
			}
			if abbSpec.Image.PullPolicy != nil && abSpec.Image.PullPolicy == nil {
				abSpec.Image.PullPolicy = abbSpec.Image.PullPolicy
			}
		}
	}

	if abbSpec.ServiceType != nil && abSpec.ServiceType == nil {
		abSpec.ServiceType = abbSpec.ServiceType
	}

	if abbSpec.Selector != nil {
		if abSpec.Selector == nil {
			abSpec.Selector = abbSpec.Selector
		} else {
			if abbSpec.Selector.MatchLabels != nil && abSpec.Selector.MatchLabels == nil {
				abSpec.Selector.MatchLabels = abbSpec.Selector.MatchLabels
			}
			if abbSpec.Selector.MatchExpressions != nil && abSpec.Selector.MatchExpressions == nil {
				abSpec.Selector.MatchExpressions = abbSpec.Selector.MatchExpressions
			}
		}
	}

	if abbSpec.LivenessProbe != nil && abSpec.LivenessProbe == nil {
		abSpec.LivenessProbe = abbSpec.LivenessProbe
	}

	if abbSpec.ReadinessProbe != nil && abSpec.ReadinessProbe == nil {
		abSpec.ReadinessProbe = abbSpec.ReadinessProbe
	}

	if abbSpec.StartupProbe != nil && abSpec.StartupProbe == nil {
		abSpec.StartupProbe = abbSpec.StartupProbe
	}

	// Recurse

	if abb.Spec.Base == nil {
		return nil
	}

	newAbb := &atroxyzv1alpha1.AppBundleBase{ObjectMeta: metav1.ObjectMeta{Name: *abb.Spec.Base}}
	err := r.Get(ctx, client.ObjectKey{Name: *abb.Spec.Base}, newAbb)

	if err != nil {
		return err
	}

	return ResolveAppBundleBase(ctx, r, ab, newAbb)
}
