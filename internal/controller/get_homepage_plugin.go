package controller

import (
	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// TODO: Remove the gethomepage.dev stuff once moved away from gethomepage.

func GetHomePageAnnotations(ingress_annotations map[string]string, ab *atroxyzv1alpha1.AppBundle) map[string]string {
	annotations := make(map[string]string)

	for key, value := range ingress_annotations {
		annotations[key] = value
	}

	annotations["gethomepage.dev/enabled"] = "true"
	annotations["atro.xyz/homepage.enabled"] = "true"

	if ab.Spec.Homepage.Description != nil {
		annotations["atro.xyz/homepage.description"] = *ab.Spec.Homepage.Description
		annotations["gethomepage.dev/description"] = *ab.Spec.Homepage.Description
	}

	if ab.Spec.Homepage.Instance != nil {
		annotations["atro.xyz/homepage.user"] = *ab.Spec.Homepage.Instance
		annotations["gethomepage.dev/instance"] = *ab.Spec.Homepage.Instance
	}

	if ab.Spec.Homepage.Group != nil {
		annotations["atro.xyz/homepage.group"] = *ab.Spec.Homepage.Group
		annotations["gethomepage.dev/group"] = *ab.Spec.Homepage.Group
	} else {
		annotations["atro.xyz/homepage.group"] = "Other"
		annotations["gethomepage.dev/group"] = "Other"
	}

	if ab.Spec.Homepage.Href != nil {
		annotations["atro.xyz/homepage.href"] = *ab.Spec.Homepage.Href
		annotations["gethomepage.dev/href"] = *ab.Spec.Homepage.Href
	}

	if ab.Spec.Homepage.Icon != nil {
		annotations["atro.xyz/homepage.icon"] = *ab.Spec.Homepage.Icon
		annotations["gethomepage.dev/icon"] = *ab.Spec.Homepage.Icon
	}

	if ab.Spec.Homepage.Name != nil {
		annotations["atro.xyz/homepage.name"] = *ab.Spec.Homepage.Name
		annotations["gethomepage.dev/name"] = *ab.Spec.Homepage.Name
	} else {
		annotations["atro.xyz/homepage.name"] = ab.Name
		annotations["gethomepage.dev/name"] = ab.Name
	}

	return annotations
}
