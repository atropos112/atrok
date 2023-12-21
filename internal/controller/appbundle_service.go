package controller

import (
	"context"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *AppBundleReconciler) ReconcileService(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("service", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(service), service)

	// REGAIN control if lost
	service.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	// CHECK and BUILD the resource
	if ab.Spec.Routes == nil && er == nil {
		// If no routes, but service exists, delete it
		if err := r.Delete(ctx, service); err != nil {
			return err
		}
		return nil
	} else if ab.Spec.Routes != nil {

		// Ports
		var ports []corev1.ServicePort
		for _, route := range ab.Spec.Routes {
			ports = append(ports, corev1.ServicePort{Name: route.Name, Port: int32(*route.Port), Protocol: "TCP"})
		}

		// Defaults to ClusterIP
		if ab.Spec.ServiceType == nil {
			ab.Spec.ServiceType = new(corev1.ServiceType)
			*ab.Spec.ServiceType = corev1.ServiceTypeClusterIP
		}

		// Labeling to match the deployment
		labels := make(map[string]string)
		if !errors.IsNotFound(er) {
			for k, v := range service.ObjectMeta.Labels {
				labels[k] = v
			}
		}
		labels["app"] = ab.Name

		annotations := make(map[string]string)
		// If no annotation, add it
		if service.Annotations != nil {
			for k, v := range service.Annotations {
				annotations[k] = v
			}
		}

		if ab.Spec.TailscaleName != nil {
			annotations["tailscale.com/hostname"] = *ab.Spec.TailscaleName
			annotations["tailscale.com/expose"] = "true"
		}

		service.ObjectMeta.Annotations = annotations

		service.Spec = corev1.ServiceSpec{
			Ports:    ports,
			Type:     *ab.Spec.ServiceType,
			Selector: map[string]string{"app": ab.Name},
		}

		if err := UpsertResource(ctx, r, service, er); err != nil {
			return err
		}
	}

	return nil
}
