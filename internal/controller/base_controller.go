package controller

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"time"

	"dario.cat/mergo"
	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
	"github.com/samber/lo"
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

// ResolveAppBundleBase resolves the base of an app bundle
func ResolveAppBundleBase(ctx context.Context, r *AppBundleReconciler, ab *atroxyzv1alpha1.AppBundle, abb *atroxyzv1alpha1.AppBundleBase) error {
	followUpAbb := abb.Spec.Base

	abbAsAb, err := abb.ToAppBundle()
	if err != nil {
		return err
	}

	if err := mergo.Merge(ab, abbAsAb, mergo.WithTransformers(mapTransformer{})); err != nil {
		return err
	}

	if followUpAbb == nil {
		return nil
	}

	newAbb := &atroxyzv1alpha1.AppBundleBase{ObjectMeta: metav1.ObjectMeta{Name: *abb.Spec.Base}}
	if r.Get(ctx, client.ObjectKey{Name: *abb.Spec.Base}, newAbb) != nil {
		return err
	}

	return ResolveAppBundleBase(ctx, r, ab, newAbb)
}

type mapTransformer struct{}

func (t mapTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Map && typ.Key().Kind() == reflect.String {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				keys := src.MapKeys()
				actualKeys := []string{}
				for _, k := range keys {
					actualKeys = append(actualKeys, k.String())
				}
				slices.Sort(actualKeys)

				for _, actKey := range actualKeys {
					k := reflect.ValueOf(actKey)
					v := src.MapIndex(k)
					exists := dst.MapIndex(k)

					// Not empty in which case look to merge
					if (exists != reflect.Value{}) && lo.Contains([]reflect.Kind{reflect.Map, reflect.Slice, reflect.Struct}, dst.MapIndex(k).Kind()) {
						vOut, err := atroxyzv1alpha1.MergeDictValues(dst.MapIndex(k).Interface(), v.Interface())
						if err != nil {
							return err
						}
						v = reflect.ValueOf(vOut)
					}

					dst.SetMapIndex(k, v)
				}
			}
			return nil
		}
	}
	return nil
}
