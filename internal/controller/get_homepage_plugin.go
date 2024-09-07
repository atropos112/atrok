package controller

import (
	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
)

// TODO: Remove the gethomepage.dev stuff once moved away from gethomepage.

func GetHomePageAnnotations(annotations map[string]string, ab *atroxyzv1alpha1.AppBundle) map[string]string {
	newAnnotations := make(map[string]string)

	for key, value := range annotations {
		newAnnotations[key] = value
	}

	newAnnotations["atro.xyz/homepage.enabled"] = "true"

	if ab.Spec.Homepage.Description != nil {
		newAnnotations["atro.xyz/homepage.description"] = *ab.Spec.Homepage.Description
	}

	if ab.Spec.Homepage.Instance != nil {
		newAnnotations["atro.xyz/homepage.user"] = *ab.Spec.Homepage.Instance
	}

	if ab.Spec.Homepage.Group != nil {
		newAnnotations["atro.xyz/homepage.group"] = *ab.Spec.Homepage.Group
	} else {
		newAnnotations["atro.xyz/homepage.group"] = "Other"
	}

	if ab.Spec.Homepage.Href != nil {
		newAnnotations["atro.xyz/homepage.href"] = *ab.Spec.Homepage.Href
	}

	if ab.Spec.Homepage.Icon != nil {
		newAnnotations["atro.xyz/homepage.icon"] = *ab.Spec.Homepage.Icon
	}

	if ab.Spec.Homepage.Name != nil {
		newAnnotations["atro.xyz/homepage.name"] = *ab.Spec.Homepage.Name
	} else {
		newAnnotations["atro.xyz/homepage.name"] = ab.Name
	}

	return newAnnotations
}
