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

	if ab.Spec.Homepage.Groups != nil {
		newAnnotations["atro.xyz/homepage.groups"] = *ab.Spec.Homepage.Groups

		// If we want user to see the page in their homepage, we want the user to have access also, assuming its not set already.
		if _, ok := newAnnotations["atro.xyz/auth.group"]; !ok {
			newAnnotations["atro.xyz/auth.group"] = *ab.Spec.Homepage.Groups
		}
	}

	if ab.Spec.Homepage.Section != nil {
		newAnnotations["atro.xyz/homepage.section"] = *ab.Spec.Homepage.Section
	} else {
		newAnnotations["atro.xyz/homepage.section"] = "Other"
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
