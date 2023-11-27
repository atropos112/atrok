package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// AppBundleBaseReconciler reconciles a AppBundleBase object
type AppBundleBaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=atro.xyz,resources=appbundlebases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundlebases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundlebases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppBundleBase object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *AppBundleBaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Get app bundle base
	abb := &atroxyzv1alpha1.AppBundleBase{}
	if err := r.Get(ctx, req.NamespacedName, abb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if abb.Status.ObservedGeneration != abb.ObjectMeta.Generation {
		abb.Status.ObservedGeneration = abb.ObjectMeta.Generation
	} else {
		return ctrl.Result{}, nil
	}

	// Get all app bundles
	abList := &atroxyzv1alpha1.AppBundleList{}

	if err := r.List(ctx, abList); err != nil {
		return ctrl.Result{}, err
	}

	for _, ab := range abList.Items {
		if ab.Spec.Base != nil && *ab.Spec.Base == abb.Name {
			ab.Status.AppBundleBaseReconciled = false
			if err := r.Status().Update(ctx, &ab); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppBundleBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&atroxyzv1alpha1.AppBundleBase{}).
		Complete(r)
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

	if abbSpec.Envs != nil {
		if abSpec.Envs == nil {
			abSpec.Envs = abbSpec.Envs
		} else {
			for key, value := range abbSpec.Envs {
				if _, ok := abSpec.Envs[key]; !ok {
					abSpec.Envs[key] = value
				}
			}
		}
	}

	if abbSpec.Routes != nil {
		if abSpec.Routes == nil {
			// If the app bundle has no routes, then we can just set it to the base routes
			abSpec.Routes = abbSpec.Routes
		} else {
			// We smart merge here,
			// Matching by names.

			for _, route := range abbSpec.Routes {
				found := false

				// Searching if ab has same name route
				for _, abRoute := range abSpec.Routes {
					if route.Name == abRoute.Name {
						found = true
						// if yes merge it
						if route.Port != nil && abRoute.Port == nil {
							abRoute.Port = route.Port
						}
						if route.Ingress != nil && abRoute.Ingress == nil {
							abRoute.Ingress = route.Ingress
						} else if route.Ingress != nil && abRoute.Ingress != nil {
							// Merge ingress
							if route.Ingress.Auth != nil && abRoute.Ingress.Auth == nil {
								abRoute.Ingress.Auth = route.Ingress.Auth
							}
							if route.Ingress.Domain != nil && abRoute.Ingress.Domain == nil {
								abRoute.Ingress.Domain = route.Ingress.Domain
							}
						}

						break
					}
				}

				// If not found, append it
				if !found {
					abSpec.Routes = append(abSpec.Routes, route)
				}
			}
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
