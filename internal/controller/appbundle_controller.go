package controller

import (
	"context"
	"fmt"

	longhornv1beta2 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// AppBundleReconciler reconciles a AppBundle object
type AppBundleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	// Just for debugging
	if app_bundle.Namespace != "devel" {
		panic("not devel namespace")
	}

	if err := r.ReconcileVolumes(ctx, req, app_bundle); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ReconcileService(ctx, req, app_bundle); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AppBundleReconciler) ReconcileService(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	// Ports
	var ports []corev1.ServicePort
	for _, route := range app_bundle.Spec.Routes {
		ports = append(ports, corev1.ServicePort{Name: route.Name, Port: int32(route.Port), Protocol: "TCP"})
	}

	if app_bundle.Spec.ServiceType == nil {
		app_bundle.Spec.ServiceType = new(corev1.ServiceType)
		*app_bundle.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: app_bundle.Name, Namespace: app_bundle.Namespace},
		Spec:       corev1.ServiceSpec{Ports: ports, Type: *app_bundle.Spec.ServiceType}}

	if err := UpsertResource(ctx, r, service); err != nil {
		return err
	}

	return nil
}

// I AM MAKING A SUPER STRONG ASSUMPTION ATM THAT ONLY RESTRICTS THIS TO PVCs (EmpytDir, HostPath, ConfigMap etc. are not supported)
func (r *AppBundleReconciler) ReconcileVolumes(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	for _, volume := range app_bundle.Spec.Volumes {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: *volume.Name, Namespace: app_bundle.Namespace},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: volume.StorageClass,
				Resources:        corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(*volume.Size)}}}}

		if err := UpsertResource(ctx, r, pvc); err != nil {
			return err
		}

		if volume.Longhorn != nil {
			reccuringJobName := fmt.Sprintf("atrok-%s-%s", app_bundle.Name, *volume.Name)
			key := fmt.Sprintf("recurring-job.longhorn.io/%s", reccuringJobName)

			if err := UpsertLabelIntoResource(ctx, r, key, "enabled", &corev1.PersistentVolumeClaim{}, types.NamespacedName{Name: *volume.Name, Namespace: app_bundle.Namespace}); err != nil {
				return err
			}
			if err := UpsertLabelIntoResource(ctx, r, "recurring-job.longhorn.io/source", "enabled", &corev1.PersistentVolumeClaim{}, types.NamespacedName{Name: *volume.Name, Namespace: app_bundle.Namespace}); err != nil {
				return err
			}

			recurringJob := &longhornv1beta2.RecurringJob{
				ObjectMeta: metav1.ObjectMeta{Name: reccuringJobName, Namespace: "longhorn-system"},
				Spec: longhornv1beta2.RecurringJobSpec{
					Name:        reccuringJobName,
					Groups:      []string{},
					Task:        longhornv1beta2.RecurringJobTypeBackup,
					Cron:        volume.Longhorn.Backup.Frequency,
					Retain:      volume.Longhorn.Backup.Retain,
					Concurrency: 1}}

			if err := UpsertResource(ctx, r, recurringJob); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *AppBundleReconciler) ReconcileDeployment(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	var ports []corev1.ContainerPort
	for _, route := range app_bundle.Spec.Routes {
		ports = append(ports, corev1.ContainerPort{Name: route.Name, HostPort: int32(route.Port), ContainerPort: int32(route.Port), Protocol: "TCP"})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app_bundle.Name, Namespace: app_bundle.Namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: app_bundle.Spec.Replicas,
			Selector: app_bundle.Spec.Selector,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
				{
					Name:  app_bundle.Name,
					Image: fmt.Sprintf("%s:%s", app_bundle.Spec.Image.Repository, app_bundle.Spec.Image.Tag),
					Ports: ports}}}}}}

	if err := UpsertResource(ctx, r, deployment); err != nil {
		return err
	}
	return nil
}
