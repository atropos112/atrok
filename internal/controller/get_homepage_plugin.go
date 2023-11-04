package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	traefikio "github.com/atropos112/atrok.git/external_apis/traefikio/v1alpha1"
)

// The recurring jobs are not cleaned up after app bundle is deleted which needs to be fixed
// GetAppBundleObjectMetaWithOwnerReference(app_bundle).OwnerReferences[] gives a list of owner references (all things i depend on) this might be useful for that
func (r *AppBundleReconciler) ReconcileHomePage(ctx context.Context, req ctrl.Request, app_bundle *atroxyzv1alpha1.AppBundle) error {
	annotations := make(map[string]string)
	ingressRoute := &traefikio.IngressRoute{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(app_bundle)}
	er := r.Get(ctx, client.ObjectKeyFromObject(ingressRoute), ingressRoute)

	for key, value := range ingressRoute.GetAnnotations() {
		annotations[key] = value
	}
	annotations["gethomepage.dev/enabled"] = "true"

	if app_bundle.Spec.Homepage.Description != nil {
		annotations["gethomepage.dev/description"] = *app_bundle.Spec.Homepage.Description
	}

	if app_bundle.Spec.Homepage.Group != nil {
		annotations["gethomepage.dev/group"] = *app_bundle.Spec.Homepage.Group
	} else {
		annotations["gethomepage.dev/group"] = "Other"
	}

	if app_bundle.Spec.Homepage.Href != nil {
		annotations["gethomepage.dev/href"] = *app_bundle.Spec.Homepage.Href
	} else {
		for _, route := range app_bundle.Spec.Routes {
			if route.Ingress != nil {
				annotations["gethomepage.dev/href"] = "https://" + route.Ingress.Domain // Domain is required so no need to check for nil
				break
			}
		}
	}

	if app_bundle.Spec.Homepage.Icon != nil {
		annotations["gethomepage.dev/icon"] = *app_bundle.Spec.Homepage.Icon
	}

	if app_bundle.Spec.Homepage.Name != nil {
		annotations["gethomepage.dev/name"] = *app_bundle.Spec.Homepage.Name
	} else {
		annotations["gethomepage.dev/name"] = app_bundle.Name
	}

	ingressRoute.SetAnnotations(annotations)

	if UpsertResource(ctx, r, ingressRoute, er) != nil {
		return er
	}

	return nil
}
