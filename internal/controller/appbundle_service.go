package controller

import (
	"context"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateExpectedService creates the expected service from the appbundle
func CreateExpectedService(ab *atroxyzv1alpha1.AppBundle) (*corev1.Service, error) {
	service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	// Ports
	var ports []corev1.ServicePort
	routeKeys := getSortedKeys(ab.Spec.Routes)
	for _, key := range routeKeys {
		route := ab.Spec.Routes[key]
		port := corev1.ServicePort{Name: key, Port: int32(*route.Port), Protocol: "TCP"}

		if route.Protocol != nil {
			port.Protocol = *route.Protocol
		}

		ports = append(ports, port)
	}

	// Defaults to ClusterIP
	if ab.Spec.ServiceType == nil {
		ab.Spec.ServiceType = new(corev1.ServiceType)
		*ab.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}

	// Labeling to match the deployment
	labels := make(map[string]string)
	labels["app"] = ab.Name

	annotations := make(map[string]string)

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
	return service, nil
}

// ReconcileService reconciles the service for the appbundle
func (r *AppBundleReconciler) ReconcileService(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("service", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET THE CURRENT SERVICE
	currentService := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentService), currentService)

	if ab.Spec.Routes == nil {
		// If there is no service and no routes on app bundle, leave now
		if errors.IsNotFound(er) {
			return nil
		}

		// If no routes, but service exists, delete it
		return r.Delete(ctx, currentService)
	}

	// GET THE EXPECTED SERVICE
	expectedService, err := CreateExpectedService(ab)
	if err != nil {
		return err
	}

	if expectedService != nil && !equality.Semantic.DeepDerivative(expectedService.Spec, currentService.Spec) {
		return UpsertResource(ctx, r, expectedService, er)
	}

	return nil
}
