package controller

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// Need to abstract this away into operator install (helm chart install)
// TESTING ONLY !!!
var (
	image_pull_secrets     []corev1.LocalObjectReference = []corev1.LocalObjectReference{{Name: "regcred"}}
	auth_middleware        string                        = "auth-authelia@kubernetescrd"
	entry_point            string                        = "websecure"
	cluster_issuer         string                        = "letsencrypt"
	base_homepage_instance string                        = "atro"
)

// TESTING ONLY !!!

// AppBundleReconciler reconciles a AppBundle object
type AppBundleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type ResourceMutexes struct {
	sync.Mutex
	m map[string]map[string]*sync.Mutex // map[resourceType][resourceName]*sync.Mutex
}

var resourceMutexes = ResourceMutexes{
	m: make(map[string]map[string]*sync.Mutex),
}

func getMutex(resourceType, name, namespace string) *sync.Mutex {
	resourceMutexes.Lock()
	defer resourceMutexes.Unlock()
	fullName := fmt.Sprintf("%s-%s", name, namespace)

	if _, ok := resourceMutexes.m[resourceType]; !ok {
		resourceMutexes.m[resourceType] = make(map[string]*sync.Mutex)
	}

	if mu, ok := resourceMutexes.m[resourceType][fullName]; ok {
		return mu
	}

	mu := &sync.Mutex{}
	resourceMutexes.m[resourceType][fullName] = mu
	return mu
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppBundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&atroxyzv1alpha1.AppBundle{}).
		Complete(r)
}
