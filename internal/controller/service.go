package controller

import (
	"context"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GeneratedServiceSpecData is data that is generated by k8s API and hence cannot be "expected" as such is passed into expected function.
type GeneratedServiceSpecData struct {
	clusterIP             string
	clusterIPs            []string
	SessionAffinity       corev1.ServiceAffinity
	IPFamilies            []corev1.IPFamily
	IPFamilyPolicy        *corev1.IPFamilyPolicy
	InternalTrafficPolicy *corev1.ServiceInternalTrafficPolicy
}

func MergeIntoServiceSpec(service *corev1.Service, data *GeneratedServiceSpecData) {
	service.Spec.ClusterIP = data.clusterIP
	service.Spec.ClusterIPs = data.clusterIPs
	service.Spec.SessionAffinity = data.SessionAffinity
	service.Spec.IPFamilies = data.IPFamilies
	service.Spec.IPFamilyPolicy = data.IPFamilyPolicy
	service.Spec.InternalTrafficPolicy = data.InternalTrafficPolicy
}

func GetGeneratedServiceSpecData(svc *corev1.Service) *GeneratedServiceSpecData {
	return &GeneratedServiceSpecData{
		clusterIP:             svc.Spec.ClusterIP,
		clusterIPs:            svc.Spec.ClusterIPs,
		SessionAffinity:       svc.Spec.SessionAffinity,
		IPFamilies:            svc.Spec.IPFamilies,
		IPFamilyPolicy:        svc.Spec.IPFamilyPolicy,
		InternalTrafficPolicy: svc.Spec.InternalTrafficPolicy,
	}
}

// CreateExpectedService creates the expected service from the appbundle
func CreateExpectedService(ab *atroxyzv1alpha1.AppBundle, generatedSpecData *GeneratedServiceSpecData) (*corev1.Service, error) {
	service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	// Ports
	var ports []corev1.ServicePort
	routeKeys := getSortedKeys(ab.Spec.Routes)
	for _, key := range routeKeys {
		route := ab.Spec.Routes[key]

		tPort := int32(*route.Port)
		if route.TargetPort != nil {
			tPort = int32(*route.TargetPort)
		}

		port := corev1.ServicePort{Name: key, Port: int32(*route.Port), TargetPort: intstr.IntOrString{IntVal: tPort}, Protocol: "TCP"}

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
	service.ObjectMeta.Labels = SetDefaultAppBundleLabels(ab, nil)

	annotations := ab.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if ab.Spec.TailscaleName != nil {
		annotations["tailscale.com/hostname"] = *ab.Spec.TailscaleName
		annotations["tailscale.com/expose"] = "true"

		if ab.Spec.Homepage != nil {
			// See if we need to add homepage annotations
			annotations = GetHomePageAnnotations(annotations, ab)
		}
	}

	service.ObjectMeta.Annotations = annotations

	service.Spec = corev1.ServiceSpec{
		Ports:    ports,
		Type:     *ab.Spec.ServiceType,
		Selector: map[string]string{AppBundleSelector: ab.Name},
	}

	MergeIntoServiceSpec(service, generatedSpecData)
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
	expectedService, err := CreateExpectedService(ab, GetGeneratedServiceSpecData(currentService))
	if err != nil {
		return err
	}

	if expectedService != nil && !equality.Semantic.DeepDerivative(expectedService.Spec, currentService.Spec) {
		reason, err := FormulateDiffMessageForSpecs(currentService.Spec, expectedService.Spec)
		if err != nil {
			return err
		}

		return UpsertResource(ctx, r, expectedService, reason, er, false)
	}

	if expectedService != nil && !StringMapsMatch(expectedService.ObjectMeta.Labels, currentService.ObjectMeta.Labels) {
		reason, err := FormulateDiffMessageForLabels(currentService.ObjectMeta.Labels, expectedService.ObjectMeta.Labels)
		if err != nil {
			return err
		}

		return UpsertResource(ctx, r, expectedService, reason, er, false)
	}

	return nil
}
