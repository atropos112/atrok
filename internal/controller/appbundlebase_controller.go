package controller

import (
	"context"
	"sync"
	"time"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	rxhash "github.com/rxwycdh/rxhash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AppBundleBaseReconciler reconciles a AppBundleBase object
type AppBundleBaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var hashedSpecAbb map[string]string = make(map[string]string)

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

	l := log.FromContext(ctx)

	// LOCK the resource
	muBase := getMutex("appBundleBase", req.Name, req.Namespace)
	muBase.Lock()
	defer muBase.Unlock()

	if req.Name != "blahtus" {
		return ctrl.Result{}, nil
	}

	// Get app bundle base
	abb := &atroxyzv1alpha1.AppBundleBase{}
	if err := r.Get(ctx, req.NamespacedName, abb); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile only if the observed generation is not the same as the current generation or
	// if the app bundle base has not been reconciled yet after it was updated
	abb_hash, err := rxhash.HashStruct(abb.Spec)
	if err != nil {
		l.Error(err, "Unable to hash app bundle.")
		return ctrl.Result{}, err
	}

	var lastRecon = time.Unix(0, 0)

	if abb.Status.LastReconciliation != nil {
		lastRecon, err = time.Parse(time.UnixDate, *abb.Status.LastReconciliation)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if hash, ok := hashedSpecAbb[abb.Name]; !ok || // If the hash is not in the map
		hash != abb_hash || // If the hash is not the same
		abb.Status.LastReconciliation == nil || // If the last reconcilliation is nil
		time.Now().Unix()-lastRecon.Unix() > 30 { // If the last reconcilliation is more than 30 seconds ago

		hashedSpecAbb[abb.Name] = abb_hash
		nowTime := time.Now().UTC().Format(time.UnixDate)
		abb.Status.LastReconciliation = &nowTime
		if err := r.Status().Update(ctx, abb); err != nil {
			l.Error(err, "Unable to update app bundle status.")
			return ctrl.Result{}, err
		}

	} else {
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	// Get the object AGAIN as we re-upserted it above.
	if err := r.Get(ctx, req.NamespacedName, abb); err != nil {
		return ctrl.Result{}, err
	}

	// Get all app bundles
	abList := &atroxyzv1alpha1.AppBundleList{}

	if err := r.List(ctx, abList); err != nil {
		return ctrl.Result{}, err
	}

	mus_ab := make(map[string]*sync.Mutex)

	for _, ab := range abList.Items {
		if ab.Spec.Base != nil && *ab.Spec.Base == abb.Name {
			mus_ab[ab.Name] = getMutex("appBundle", ab.Name, ab.Namespace)
			mus_ab[ab.Name].Lock()
			hashedSpecAb[ab.Name] = "" // Force update by reseting the hashed spec
			if err := r.Status().Update(ctx, &ab); err != nil {
				return ctrl.Result{}, err
			}

			mus_ab[ab.Name].Unlock()
		}
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppBundleBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&atroxyzv1alpha1.AppBundleBase{}).
		Complete(r)
}

// ResolveAppBundleBase
func ResolveAppBundleBase(ctx context.Context, r *AppBundleReconciler, ab *atroxyzv1alpha1.AppBundle, abb *atroxyzv1alpha1.AppBundleBase) error {
	abSpec := &ab.Spec
	abbSpec := &abb.Spec

	// By hand merge, can do with reflection but then its not clear when to override, when to append etc.

	if abb.Spec.Command != nil && abSpec.Command == nil {
		abSpec.Command = abbSpec.Command
	}

	if abbSpec.Args != nil && abSpec.Args == nil {
		abSpec.Args = abbSpec.Args
	}

	if abbSpec.Volumes != nil {

		if abSpec.Volumes == nil {
			abSpec.Volumes = abbSpec.Volumes
		} else {
			abbVolumeKeys := getSortedKeys(abbSpec.Volumes)

			for _, abbVolKey := range abbVolumeKeys {
				abbVol := abbSpec.Volumes[abbVolKey]

				if abVol, ok := abSpec.Volumes[abbVolKey]; ok {
					if abbVol.Path != nil && abVol.Path == nil {
						abVol.Path = abbVol.Path
					}
					if abbVol.Size != nil && abVol.Size == nil {
						abVol.Size = abbVol.Size
					}
					if abbVol.StorageClass != nil && abVol.StorageClass == nil {
						abVol.StorageClass = abbVol.StorageClass
					}
					if abbVol.ExistingClaim != nil && abVol.ExistingClaim == nil {
						abVol.ExistingClaim = abbVol.ExistingClaim
					}
					if abbVol.Backup != nil && abVol.Backup == nil {
						abVol.Backup = abbVol.Backup
					}
					if abbVol.HostPath != nil && abVol.HostPath == nil {
						abVol.HostPath = abbVol.HostPath
					}
					if abbVol.EmptyDir != nil && abVol.EmptyDir == nil {
						abVol.EmptyDir = abbVol.EmptyDir
					}
					abSpec.Volumes[abbVolKey] = abVol
				} else {
					abSpec.Volumes[abbVolKey] = abbVol
				}
			}
		}
	}

	if abbSpec.Backup != nil {
		if abSpec.Backup == nil {
			abSpec.Backup = abbSpec.Backup
		} else {
			if abbSpec.Backup.Frequency != nil && abSpec.Backup.Frequency == nil {
				abSpec.Backup.Frequency = abbSpec.Backup.Frequency
			}
			if abbSpec.Backup.Retain != nil && abSpec.Backup.Retain == nil {
				abSpec.Backup.Retain = abbSpec.Backup.Retain
			}
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

	if abbSpec.SourcedEnvs != nil {
		if abSpec.SourcedEnvs == nil {
			abSpec.SourcedEnvs = abbSpec.SourcedEnvs
		} else {
			for key, value := range abbSpec.SourcedEnvs {
				if _, ok := abSpec.SourcedEnvs[key]; !ok {
					abSpec.SourcedEnvs[key] = value
				}
			}
		}
	}

	if abbSpec.NodeSelector != nil && abSpec.NodeSelector == nil {
		abSpec.NodeSelector = abbSpec.NodeSelector
	}

	if abbSpec.UseNvidia != nil && abSpec.UseNvidia == nil {
		abSpec.UseNvidia = abbSpec.UseNvidia
	}

	if abbSpec.Routes != nil {
		if abSpec.Routes == nil {
			// If the app bundle has no routes, then we can just set it to the base routes
			abSpec.Routes = abbSpec.Routes
		} else {
			abbRouteKeys := getSortedKeys(abbSpec.Routes)

			for _, key := range abbRouteKeys {
				abbRoute := abbSpec.Routes[key]

				if abRoute, ok := abSpec.Routes[key]; ok {
					if abbRoute.Port != nil && abRoute.Port == nil {
						abRoute.Port = abbRoute.Port
					}
					if abbRoute.Protocol != nil && abRoute.Protocol == nil {
						abRoute.Protocol = abbRoute.Protocol
					}
					if abbRoute.Ingress != nil && abRoute.Ingress == nil {
						abRoute.Ingress = abbRoute.Ingress
					} else if abbRoute.Ingress != nil && abRoute.Ingress != nil {
						// Merge ingress
						if abbRoute.Ingress.Auth != nil && abRoute.Ingress.Auth == nil {
							abRoute.Ingress.Auth = abbRoute.Ingress.Auth
						}
						if abbRoute.Ingress.Domain != nil && abRoute.Ingress.Domain == nil {
							abRoute.Ingress.Domain = abbRoute.Ingress.Domain
						}
					}
					abSpec.Routes[key] = abRoute
				} else {
					abSpec.Routes[key] = abbRoute
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

		if abbSpec.Homepage.Instance != nil && abSpec.Homepage.Instance == nil {
			abSpec.Homepage.Instance = abbSpec.Homepage.Instance
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
