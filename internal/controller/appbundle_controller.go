package controller

import (
	"context"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	traefikio "github.com/atropos112/atrok.git/external_apis/traefikio/v1alpha1"
)

// Need to abstract this away into operator install (helm chart install)
// TESTING ONLY !!!
var (
	image_pull_secrets []corev1.LocalObjectReference = []corev1.LocalObjectReference{{Name: "regcred"}}
	auth_middleware    traefikio.MiddlewareRef       = traefikio.MiddlewareRef{Name: "authelia", Namespace: "auth"}
	entry_points       []string                      = []string{"websecure"}
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

//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=atro.xyz,resources=appbundles/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *AppBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Get app bundle
	app_bundle := &atroxyzv1alpha1.AppBundle{}
	if err := r.Get(ctx, req.NamespacedName, app_bundle); err != nil {
		l.Error(err, "Unable to fetch app bundle, it was probably deleted, if not its a problem.")
		return ctrl.Result{}, err
	}
	if app_bundle.Status.ObservedGeneration != app_bundle.ObjectMeta.Generation {
		app_bundle.Status.ObservedGeneration = app_bundle.ObjectMeta.Generation
		if err := r.Status().Update(ctx, app_bundle); err != nil {
			l.Error(err, "Unable to update app bundle status.")
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, nil
	}

	if err := RunReconciles(ctx, r, req, app_bundle,
		//r.ReconcileVolumes,
		//r.ReconcileService,
		r.ReconcileDeployment,
		//r.ReconcileIngressRoute,
	); err != nil {
		l.Error(err, "Unable to reconcile app bundle.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AppBundleReconciler) ReconcileService(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("service", app_bundle.Name, app_bundle.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	service := &corev1.Service{}
	er := r.Get(ctx, client.ObjectKey{Name: app_bundle.Name, Namespace: app_bundle.Namespace}, service)

	// CHECK and BUILD the resource
	if app_bundle.Spec.Routes == nil && er == nil {
		// If no routes, but service exists, delete it
		if err := r.Delete(ctx, service); err != nil {
			return err
		}
		return nil
	} else if app_bundle.Spec.Routes != nil {

		// Ports
		var ports []corev1.ServicePort
		for _, route := range app_bundle.Spec.Routes {
			ports = append(ports, corev1.ServicePort{Name: route.Name, Port: int32(route.Port), Protocol: "TCP"})
		}

		// Defaults to ClusterIP
		if app_bundle.Spec.ServiceType == nil {
			app_bundle.Spec.ServiceType = new(corev1.ServiceType)
			*app_bundle.Spec.ServiceType = corev1.ServiceTypeClusterIP
		}

		// Labeling to match the deployment
		labels := make(map[string]string)
		if !errors.IsNotFound(er) {
			labels = app_bundle.GetLabels()
		}
		labels["app"] = app_bundle.Name

		service.Spec = corev1.ServiceSpec{
			Ports:    ports,
			Type:     *app_bundle.Spec.ServiceType,
			Selector: map[string]string{"app": app_bundle.Name},
		}

		if err := UpsertResource(ctx, r, service, er); err != nil {
			return err
		}
	}

	return nil
}

// I AM MAKING A SUPER STRONG ASSUMPTION ATM THAT ONLY RESTRICTS THIS TO PVCs (EmpytDir, HostPath, ConfigMap etc. are not supported)
func (r *AppBundleReconciler) ReconcileVolumes(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("volume", app_bundle.Name, app_bundle.Namespace)
	mu.Lock()
	defer mu.Unlock()

	if app_bundle.Spec.Volumes == nil {
		return nil
	}
	for _, volume := range app_bundle.Spec.Volumes {
		// CHECK if we need to continue
		if volume.Longhorn == nil && volume.ExistingClaim == nil {
			// If its an existing claim, and no longhorn plugin is used, we leave it alone
			return nil
		}

		// GET the resource
		objMeta := GetAppBundleObjectMetaWithOwnerReference(app_bundle)
		objMeta.Name = *volume.Name
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: objMeta}

		er := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)

		// If not existing claim, its up to us to create and manage it
		// Can only change spec of an existing PVC
		if volume.ExistingClaim == nil && errors.IsNotFound(er) {
			// BUILD the resource
			pvc.Spec = corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: volume.StorageClass,
				Resources:        corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(*volume.Size)}}}

			// UPSERT the resource
			if err := UpsertResource(ctx, r, pvc, er); err != nil {
				return err
			}
		}

		// LONGHORN backup plugin reconciliation
		if volume.Longhorn != nil {
			if err := r.ReconcileBackup(ctx, req, app_bundle, volume); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *AppBundleReconciler) ReconcileDeployment(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("deployment", app_bundle.Name, app_bundle.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(app_bundle)}
	er := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)

	// CHECK and BUILD the resource

	// Ports
	var ports []corev1.ContainerPort
	for _, route := range app_bundle.Spec.Routes {
		ports = append(ports, corev1.ContainerPort{Name: route.Name, HostPort: int32(route.Port), ContainerPort: int32(route.Port), Protocol: "TCP"})
	}

	// Volume Mounts
	var volume_mounts []corev1.VolumeMount
	for _, volume := range app_bundle.Spec.Volumes {
		volume_mounts = append(volume_mounts, corev1.VolumeMount{Name: *volume.Name, MountPath: *volume.Path})
	}

	// Volumes
	var volumes []corev1.Volume
	for _, volume := range app_bundle.Spec.Volumes {
		volumes = append(volumes, corev1.Volume{Name: *volume.Name, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: *volume.Name}}})
	}

	// Small bits
	revision_history_limit := int32(3)
	labels := make(map[string]string)
	for key, value := range app_bundle.GetLabels() {
		labels[key] = value
	}
	labels["app"] = app_bundle.Name

	resources := corev1.ResourceRequirements{}
	if app_bundle.Spec.Resources != nil {
		resources = *app_bundle.Spec.Resources
	}

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             app_bundle.Spec.Replicas,
		RevisionHistoryLimit: &revision_history_limit,
		Strategy:             appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
		Selector:             &metav1.LabelSelector{MatchLabels: map[string]string{"app": app_bundle.Name}},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Volumes:          volumes,
				ImagePullSecrets: image_pull_secrets,
				Containers: []corev1.Container{
					{
						Name:           app_bundle.Name,
						Image:          fmt.Sprintf("%s:%s", app_bundle.Spec.Image.Repository, app_bundle.Spec.Image.Tag),
						Resources:      resources,
						Ports:          ports,
						VolumeMounts:   volume_mounts,
						LivenessProbe:  app_bundle.Spec.LivenessProbe,
						ReadinessProbe: app_bundle.Spec.ReadinessProbe,
						StartupProbe:   app_bundle.Spec.StartupProbe,
					}}}}}

	// UPSERT the resource
	if err := UpsertResource(ctx, r, deployment, er); err != nil {
		return err
	}

	return nil
}

func (r *AppBundleReconciler) ReconcileIngressRoute(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("ingress_route", app_bundle.Name, app_bundle.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	ingress_route := &traefikio.IngressRoute{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(app_bundle)}
	er := r.Get(ctx, client.ObjectKeyFromObject(ingress_route), ingress_route)

	// CHECK and BUILD the resource

	// If no routes, but ingress exists, delete it
	if app_bundle.Spec.Routes == nil && er == nil {

		if err := r.Delete(ctx, ingress_route); err != nil {
			return err
		}
		return nil
		// If routes exist and ingress exists, update it
	} else if app_bundle.Spec.Routes != nil {

		// BUILD the resource
		routes := []traefikio.Route{}
		for _, route := range app_bundle.Spec.Routes {
			middlewares := []traefikio.MiddlewareRef{}
			if route.Ingress.Auth {
				middlewares = append(middlewares, auth_middleware)
			}

			service := traefikio.LoadBalancerSpec{Name: app_bundle.Name, Port: intstr.IntOrString{IntVal: int32(route.Port)}}
			routes = append(routes, traefikio.Route{
				Match:       fmt.Sprintf("Host(`%s`)", route.Ingress.Domain),
				Kind:        "Rule",
				Services:    []traefikio.Service{{LoadBalancerSpec: service}},
				Middlewares: middlewares,
			})
		}

		ingress_route.Spec = traefikio.IngressRouteSpec{
			EntryPoints: entry_points,
			Routes:      routes,
			TLS:         &traefikio.TLS{SecretName: fmt.Sprintf("%s-%s", app_bundle.Name, app_bundle.Namespace)},
		}

		// UPSERT the resource
		if err := UpsertResource(ctx, r, ingress_route, er); err != nil {
			return err
		}

		if app_bundle.Spec.Homepage != nil {
			if err := r.ReconcileHomePage(ctx, req, app_bundle); err != nil {
				return err
			}
		}

	}

	return nil
}
