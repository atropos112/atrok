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
	for _, key := range getSortedKeys(ab.Spec.Configs) {
		config := ab.Spec.Configs[key]

		if config.Existing != nil {
			continue
		}

		if len(config.Secrets) != 0 {
			continue
		}

		cm.Data[key] = config.Content
	}

	if len(cm.Data) == 0 {
		return nil, nil
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

	// GET THE EXPECTED CONFIGMAP
	expectedConfigMap, err := CreateExpectedConfigMap(ab)
	if err != nil {
		return err
	}

	// If expected to have no config map
	if expectedConfigMap == nil {
		// If we have an error and its not the not found error return now, something went wrong
		if er != nil && !errors.IsNotFound(er) {
			return er
		}

		// We expect to have no config map and indeed have no config map just return nil
		if errors.IsNotFound(er) {
			return nil
		}

		// Expected to have no config map but have one, delete it
		return r.Delete(ctx, currentConfigMap)
	}

	if expectedConfigMap != nil && !equality.Semantic.DeepDerivative(expectedConfigMap.Data, currentConfigMap.Data) {
		reason := "Data in the ConfigMap " + ab.Name + " has changed."

		if err := UpsertResource(ctx, r, expectedConfigMap, reason, er, false); err != nil {
			return err
		}

		// Restart the pod after the config map has been updated.
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.MatchingLabels{AppBundleSelector: ab.Name}); err != nil {
			return err
		}
		if len(podList.Items) == 0 {
			return nil
		} else if len(podList.Items) > 1 {
			return errors.NewBadRequest("More than one pod found for appbundle")
		}

		// By now we know there is only one item in the list
		if err := r.Delete(ctx, &podList.Items[0]); err != nil {
			return err
		}
	}
	return nil
}
