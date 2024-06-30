package controller

import (
	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

func GetHomePageAnnotations(ingress_annotations map[string]string, ab *atroxyzv1alpha1.AppBundle) map[string]string {
	annotations := make(map[string]string)

	for key, value := range ingress_annotations {
		annotations[key] = value
	}

	annotations["gethomepage.dev/enabled"] = "true"

	if ab.Spec.Homepage.Description != nil {
		annotations["gethomepage.dev/description"] = *ab.Spec.Homepage.Description
	}

	if ab.Spec.Homepage.Instance != nil {
		annotations["gethomepage.dev/instance"] = *ab.Spec.Homepage.Instance
	}

	if ab.Spec.Homepage.Group != nil {
		annotations["gethomepage.dev/group"] = *ab.Spec.Homepage.Group
	} else {
		annotations["gethomepage.dev/group"] = "Other"
	}

	if ab.Spec.Homepage.Href != nil {
		annotations["gethomepage.dev/href"] = *ab.Spec.Homepage.Href
	}

	if ab.Spec.Homepage.Icon != nil {
		annotations["gethomepage.dev/icon"] = *ab.Spec.Homepage.Icon
	}

	if ab.Spec.Homepage.Name != nil {
		annotations["gethomepage.dev/name"] = *ab.Spec.Homepage.Name
	} else {
		annotations["gethomepage.dev/name"] = ab.Name
	}

	return annotations
}
