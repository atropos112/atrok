package controller

import (
	"context"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// The recurring jobs are not cleaned up after app bundle is deleted which needs to be fixed
// GetAppBundleObjectMetaWithOwnerReference(ab).OwnerReferences[] gives a list of owner references (all things i depend on) this might be useful for that
func (r *AppBundleReconciler) ReconcileHomePage(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	annotations := make(map[string]string)
	ingress := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
	// REGAIN control if lost
	ingress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	for key, value := range ingress.GetAnnotations() {
		annotations[key] = value
	}
	annotations["gethomepage.dev/enabled"] = "true"

	if ab.Spec.Homepage.Description != nil {
		annotations["gethomepage.dev/description"] = *ab.Spec.Homepage.Description
	}

	if ab.Spec.Homepage.Group != nil {
		annotations["gethomepage.dev/group"] = *ab.Spec.Homepage.Group
	} else {
		annotations["gethomepage.dev/group"] = "Other"
	}

	if ab.Spec.Homepage.Href != nil {
		annotations["gethomepage.dev/href"] = *ab.Spec.Homepage.Href
	} else {
		for _, route := range ab.Spec.Routes {
			if route.Ingress != nil {
				annotations["gethomepage.dev/href"] = "https://" + *route.Ingress.Domain // Domain is required so no need to check for nil
				break
			}
		}
	}

	if ab.Spec.Homepage.Icon != nil {
		annotations["gethomepage.dev/icon"] = *ab.Spec.Homepage.Icon
	}

	if ab.Spec.Homepage.Name != nil {
		annotations["gethomepage.dev/name"] = *ab.Spec.Homepage.Name
	} else {
		annotations["gethomepage.dev/name"] = ab.Name
	}

	ingress.SetAnnotations(annotations)

	if UpsertResource(ctx, r, ingress, er) != nil {
		return er
	}

	return nil
}
