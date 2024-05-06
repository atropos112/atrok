package controller

import (
	"context"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateExpectedConfigMap creates the expected config mapfrom the appbundle
func CreateExpectedConfigMap(ab *atroxyzv1alpha1.AppBundle) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}

	// If no configs, return nil
	if ab.Spec.Configs == nil {
		return nil, nil
	}

	// Trivail mappings.
	cm.Data = make(map[string]string)
	for k, v := range ab.Spec.Configs {
		cm.Data[k] = v.Data
	}

	return cm, nil
}

// ReconcileConfigMap the config map for the appbundle
func (r *AppBundleReconciler) ReconcileConfigMap(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("configmap", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET THE CURRENT CONFIGMAP
	currentConfigMap := &corev1.ConfigMap{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentConfigMap), currentConfigMap)

	// If configmap is found but the spec is nil, delete the configmap
	if ab.Spec.Configs == nil {
		// If there is no config map and no configs on app bundle, leave now
		if errors.IsNotFound(er) {
			return nil
		}

		// If no configs, but config map exists, delete it
		return r.Delete(ctx, currentConfigMap)
	}

	// GET THE EXPECTED CONFIGMAP
	expectedConfigMap, err := CreateExpectedConfigMap(ab)
	if err != nil {
		return err
	}

	if expectedConfigMap != nil && !equality.Semantic.DeepDerivative(expectedConfigMap.Data, currentConfigMap.Data) {
		reason := "Data in the ConfigMap " + ab.Name + " has changed."
		if err != nil {
			return err
		}

		return UpsertResource(ctx, r, expectedConfigMap, reason, er)
	}
	return nil
}
