package controller

import (
	"context"
	"reflect"
	"sync"
	"time"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
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

type AppBundleIdentifier string // Identifier for the app bundle

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

	// Get app bundle base
	abb := &atroxyzv1alpha1.AppBundleBase{}
	if err := r.Get(ctx, req.NamespacedName, abb); err != nil {
		return ctrl.Result{}, err
	}

	// Get (cached) state of the appbundlebase
	err := RegisterStateIfNotAlreadyRegistered(abb)
	stateAlreadyRegistered := false
	if err != nil {
		_, stateAlreadyRegistered = err.(StateAlreadyRegisteredError)
		if !stateAlreadyRegistered {
			// Already registered is ok, any other error, return and requeue
			return ctrl.Result{RequeueAfter: 20 * time.Second}, err
		}
	}

	stateNeedsUpdating, err := StateNeedsUpdating(abb, stateAlreadyRegistered)
	if err != nil {
		return ctrl.Result{RequeueAfter: 20 * time.Second}, err
	}

	if !stateNeedsUpdating {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
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
			stateAb, err := GetState(ab)
			if err != nil {
				mus_ab[ab.Name].Unlock()
				if _, ok := err.(StateNotRegisteredError); !ok {
					return ctrl.Result{}, err
				} else {
					l.Info("State not registered for appbundle. This is normal at start, should not be happening when operator ahs been running for a while though. Requeueing", "appbundle", ab.Name)
					// In case we go here before state was recorder for the appbundle itself.
					return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
				}
			}
			stateAb.SpecHash = "" // Force update by resetting the hashed spec
			abID := AppBundleIdentifier(ab.ID())
			ctx = context.WithValue(ctx, abID, stateAb)

			if err := r.Status().Update(ctx, &ab); err != nil {
				mus_ab[ab.Name].Unlock()
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

func IsDefault[T any](value T) bool {
	defaultValue := reflect.Zero(reflect.TypeOf(value)).Interface()
	return reflect.DeepEqual(value, defaultValue)
}

func SetDefault[T any](value *T) {
	*value = reflect.Zero(reflect.TypeOf(*value)).Interface().(T)
}

// ReturnFirstNonNil returns the first non-nil element in a list of pointers
func ReturnFirstNonDefault[T any](elem ...T) T {
	var result T
	for _, e := range elem {
		if !IsDefault[T](e) {
			return e
		}
	}
	return result
}

// ResolveAppBundleBase
func ResolveAppBundleBase(ctx context.Context, r *AppBundleReconciler, ab *atroxyzv1alpha1.AppBundle, abb *atroxyzv1alpha1.AppBundleBase) error {
	abSpec := &ab.Spec
	abbSpec := &abb.Spec

	// By hand merge, can do with reflection but then its not clear when to override, when to append etc.
	abSpec.Command = ReturnFirstNonDefault(abSpec.Command, abbSpec.Command)
	abbSpec.Args = ReturnFirstNonDefault(abSpec.Args, abbSpec.Args)

	if abbSpec.Volumes != nil {
		if abSpec.Volumes == nil {
			abSpec.Volumes = abbSpec.Volumes
		} else {
			abbVolumeKeys := getSortedKeys(abbSpec.Volumes)

			for _, abbVolKey := range abbVolumeKeys {
				abbVol := abbSpec.Volumes[abbVolKey]

				if abVol, ok := abSpec.Volumes[abbVolKey]; ok {
					abVol.Path = ReturnFirstNonDefault(abVol.Path, abbVol.Path)
					abVol.Size = ReturnFirstNonDefault(abVol.Size, abbVol.Size)
					abVol.StorageClass = ReturnFirstNonDefault(abVol.StorageClass, abbVol.StorageClass)
					abVol.ExistingClaim = ReturnFirstNonDefault(abVol.ExistingClaim, abbVol.ExistingClaim)
					abVol.Backup = ReturnFirstNonDefault(abVol.Backup, abbVol.Backup)
					abVol.HostPath = ReturnFirstNonDefault(abVol.HostPath, abbVol.HostPath)
					abVol.EmptyDir = ReturnFirstNonDefault(abVol.EmptyDir, abbVol.EmptyDir)

					abSpec.Volumes[abbVolKey] = abVol
				} else {
					abSpec.Volumes[abbVolKey] = abbVol
				}
			}
		}
	}

	if abbSpec.Configs != nil {
		if abSpec.Configs == nil {
			abSpec.Configs = abbSpec.Configs
		} else {
			for _, abbKey := range getSortedKeys(abbSpec.Configs) {
				found := false
				abbConfig := abbSpec.Configs[abbKey]

				var foundConfig *atroxyzv1alpha1.AppBundleConfig
				for _, abKey := range getSortedKeys(abSpec.Configs) {
					if abKey == abbKey {
						found = true
						foundConfig = abSpec.Configs[abKey]
						break
					}
				}

				if !found {
					abSpec.Configs[abbKey] = abbConfig
				} else {
					abSpec.Configs[abbKey].Content = ReturnFirstNonDefault(foundConfig.Content, abbConfig.Content)
					abSpec.Configs[abbKey].DirPath = ReturnFirstNonDefault(foundConfig.DirPath, abbConfig.DirPath)
					abSpec.Configs[abbKey].Existing = ReturnFirstNonDefault(foundConfig.Existing, abbConfig.Existing)
					abSpec.Configs[abbKey].CopyOver = ReturnFirstNonDefault(foundConfig.CopyOver, abbConfig.CopyOver)
					for key, value := range abbConfig.Secrets {
						abSpec.Configs[abbKey].Secrets[key] = ReturnFirstNonDefault(foundConfig.Secrets[key], value)
					}
				}
			}
		}
	}

	if abbSpec.SecretStoreRef != nil {
		abSpec.SecretStoreRef = ReturnFirstNonDefault(abSpec.SecretStoreRef, abbSpec.SecretStoreRef)
	}

	abSpec.NodeSelector = ReturnFirstNonDefault(abSpec.NodeSelector, abbSpec.NodeSelector)

	if abbSpec.Backup != nil {
		if abSpec.Backup == nil {
			abSpec.Backup = abbSpec.Backup
		} else {
			abSpec.Backup.Frequency = ReturnFirstNonDefault(abSpec.Backup.Frequency, abbSpec.Backup.Frequency)
			abSpec.Backup.Retain = ReturnFirstNonDefault(abSpec.Backup.Retain, abbSpec.Backup.Retain)
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

	abSpec.NodeSelector = ReturnFirstNonDefault(abSpec.NodeSelector, abbSpec.NodeSelector)
	abSpec.UseNvidia = ReturnFirstNonDefault(abSpec.UseNvidia, abbSpec.UseNvidia)

	if abbSpec.Routes != nil {
		if abSpec.Routes == nil {
			// If the app bundle has no routes, then we can just set it to the base routes
			abSpec.Routes = abbSpec.Routes
		} else {
			abbRouteKeys := getSortedKeys(abbSpec.Routes)

			for _, key := range abbRouteKeys {
				abbRoute := abbSpec.Routes[key]

				if abRoute, ok := abSpec.Routes[key]; ok {
					abRoute.Port = ReturnFirstNonDefault(abRoute.Port, abbRoute.Port)
					abRoute.TargetPort = ReturnFirstNonDefault(abRoute.TargetPort, abbRoute.TargetPort)
					abRoute.Protocol = ReturnFirstNonDefault(abRoute.Protocol, abbRoute.Protocol)

					if abbRoute.Ingress != nil && abRoute.Ingress == nil {
						abRoute.Ingress = abbRoute.Ingress
					} else if abbRoute.Ingress != nil && abRoute.Ingress != nil {
						abRoute.Ingress.Auth = ReturnFirstNonDefault(abRoute.Ingress.Auth, abbRoute.Ingress.Auth)
						abRoute.Ingress.Domain = ReturnFirstNonDefault(abRoute.Ingress.Domain, abbRoute.Ingress.Domain)
					}
					abSpec.Routes[key] = abRoute
				} else {
					abSpec.Routes[key] = abbRoute
				}
			}
		}
	}

	abSpec.Resources = ReturnFirstNonDefault(abSpec.Resources, abbSpec.Resources)
	abSpec.Replicas = ReturnFirstNonDefault(abSpec.Replicas, abbSpec.Replicas)

	// Special case, happy to fill in the blanks but not the whole things,
	// it makes no sense to inherit the whole thing, so it needs to exist in some capacity beforehand
	if abbSpec.Homepage != nil && abSpec.Homepage != nil {
		abSpec.Homepage.Name = ReturnFirstNonDefault(abSpec.Homepage.Name, abbSpec.Homepage.Name)
		abSpec.Homepage.Description = ReturnFirstNonDefault(abSpec.Homepage.Description, abbSpec.Homepage.Description)
		abSpec.Homepage.Group = ReturnFirstNonDefault(abSpec.Homepage.Group, abbSpec.Homepage.Group)
		abSpec.Homepage.Href = ReturnFirstNonDefault(abSpec.Homepage.Href, abbSpec.Homepage.Href)
		abSpec.Homepage.Icon = ReturnFirstNonDefault(abSpec.Homepage.Icon, abbSpec.Homepage.Icon)
		abSpec.Homepage.Instance = ReturnFirstNonDefault(abSpec.Homepage.Instance, abbSpec.Homepage.Instance)
	}

	if abbSpec.Image != nil {
		if abSpec.Image == nil {
			abSpec.Image = &atroxyzv1alpha1.AppBundleImage{
				Repository: abbSpec.Image.Repository,
				Tag:        abbSpec.Image.Tag,
				PullPolicy: abbSpec.Image.PullPolicy,
			}
		} else {
			abSpec.Image.Repository = ReturnFirstNonDefault(abSpec.Image.Repository, abbSpec.Image.Repository)
			abSpec.Image.Tag = ReturnFirstNonDefault(abSpec.Image.Tag, abbSpec.Image.Tag)
			abSpec.Image.PullPolicy = ReturnFirstNonDefault(abSpec.Image.PullPolicy, abbSpec.Image.PullPolicy)
		}
	}

	abSpec.ServiceType = ReturnFirstNonDefault(abSpec.ServiceType, abbSpec.ServiceType)

	if abbSpec.Selector != nil {
		if abSpec.Selector == nil {
			abSpec.Selector = abbSpec.Selector
		} else {
			abSpec.Selector.MatchLabels = ReturnFirstNonDefault(abSpec.Selector.MatchLabels, abbSpec.Selector.MatchLabels)
			abSpec.Selector.MatchExpressions = ReturnFirstNonDefault(abSpec.Selector.MatchExpressions, abbSpec.Selector.MatchExpressions)
		}
	}

	abSpec.LivenessProbe = ReturnFirstNonDefault(abSpec.LivenessProbe, abbSpec.LivenessProbe)
	abSpec.ReadinessProbe = ReturnFirstNonDefault(abSpec.ReadinessProbe, abbSpec.ReadinessProbe)
	abSpec.StartupProbe = ReturnFirstNonDefault(abSpec.StartupProbe, abbSpec.StartupProbe)

	// Recurse
	if abb.Spec.Base == nil {
		return nil
	}

	newAbb := &atroxyzv1alpha1.AppBundleBase{ObjectMeta: metav1.ObjectMeta{Name: *abb.Spec.Base}}
	err := r.Get(ctx, client.ObjectKey{Name: *abb.Spec.Base}, newAbb)
	if err != nil {
		return err
	}

	labels := ab.ObjectMeta.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	if _, ok := labels["atro.xyz/app-bundle-bases"]; !ok {
		labels["atro.xyz/app-bundle-bases"] = abb.Name
	} else {
		labels["atro.xyz/app-bundle-bases"] += "." + abb.Name
	}

	return ResolveAppBundleBase(ctx, r, ab, newAbb)
}
