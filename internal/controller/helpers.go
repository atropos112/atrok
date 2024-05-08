package controller

import (
	"context"
	"reflect"
	"sort"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	"github.com/r3labs/diff/v3"
	"golang.org/x/sync/errgroup"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReaderWriter interface {
	client.Reader
	client.Writer
}

// UpsertLabel of an existing object
func UpsertLabelIntoResource(ctx context.Context, r ReaderWriter, kv map[string]string, obj client.Object, id client.ObjectKey) error {
	if err := r.Get(ctx, id, obj); err != nil {
		return err
	}

	labels := obj.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range kv {
		labels[key] = value
	}

	obj.SetLabels(labels)

	if err := r.Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func GetDiffPaths(oldObj, newObj interface{}) (string, error) {
	changes, err := diff.Diff(oldObj, newObj)
	if err != nil {
		return "", err
	}

	paths := []string{}
	for _, change := range changes {
		path := ""
		for _, p := range change.Path {
			path += p + "/"
		}
		paths = append(paths, path)
	}

	output := "\n"
	for _, path := range paths {
		output += path + "\n"
	}

	return output, nil
}

func FormulateDiffMessageForSpecs(oldObjSpec, newObjSpec interface{}) (string, error) {
	diff, err := GetDiffPaths(oldObjSpec, newObjSpec)
	if err != nil {
		return "", err
	}
	reason := "Spec changed, namely the paths: " + diff
	return reason, nil
}

func FormulateDiffMessageForLabels(oldObjLabels, newObjLabels interface{}) (string, error) {
	diff, err := GetDiffPaths(oldObjLabels, newObjLabels)
	if err != nil {
		return "", err
	}
	reason := "Labels changed, namely the paths: " + diff
	return reason, nil
}

// UpsertResource creates or updates a resource with nice logging indicating what is happening.
func UpsertResource(ctx context.Context, r ReaderWriter, newObj client.Object, reason string, er error) error {
	l := log.FromContext(ctx)

	if er != nil && !k8serror.IsNotFound(er) {
		return er
	}

	if reason != "" {
		l.Info("Upserting reason because: " + reason)
	}

	if k8serror.IsNotFound(er) {
		l.Info("Creating resource.", "type", reflect.TypeOf(newObj).String(), "object", newObj)
		if err := r.Create(ctx, newObj); err != nil {
			return err
		}
	} else {
		l.Info("Resource exists but changes were found.", "type", reflect.TypeOf(newObj).String(), "object", newObj)
		if err := r.Update(ctx, newObj); err != nil {
			return err
		}
	}

	return nil
}

// RunReconciles takes in a list of reconcile functions, passes argument into each one and runs concurrently.
func RunReconciles(
	ctx context.Context,
	app_bundle *atroxyzv1alpha1.AppBundle,
	reconciles ...func(context.Context, *atroxyzv1alpha1.AppBundle) error,
) error {
	errs, ctx := errgroup.WithContext(ctx)

	for _, reconcile := range reconciles {
		currentReconcile := reconcile // Capture current value of reconcile
		errs.Go(func() error { return currentReconcile(ctx, app_bundle) })
	}

	return errs.Wait()
}

func GetAppBundleObjectMetaWithOwnerReference(app_bundle *atroxyzv1alpha1.AppBundle) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:            app_bundle.Name,
		Namespace:       app_bundle.Namespace,
		Labels:          app_bundle.GetLabels(),
		OwnerReferences: []metav1.OwnerReference{app_bundle.OwnerReference()},
	}
}

func GetAppBundleObjectMetaWithOwnerReferenceForIngress(app_bundle *atroxyzv1alpha1.AppBundle) metav1.ObjectMeta {
	firstKey := getSortedKeys(app_bundle.Spec.Routes)[0]
	return metav1.ObjectMeta{
		Name:            app_bundle.Name + "-" + firstKey,
		Namespace:       app_bundle.Namespace,
		Labels:          app_bundle.GetLabels(),
		OwnerReferences: []metav1.OwnerReference{app_bundle.OwnerReference()},
	}
}

func GetAppBundleNamespacedName(ab *atroxyzv1alpha1.AppBundle) types.NamespacedName {
	return types.NamespacedName{Name: ab.Name, Namespace: ab.Namespace}
}

// Function to get sorted keys from a map with string keys
func getSortedKeys[V any](m map[string]V) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// SetDefaultAppBundleLabels attaches default labels to a derivative object of an app bundle
func SetDefaultAppBundleLabels(ab *atroxyzv1alpha1.AppBundle, labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	// Force overwrite if set by user.
	labels["app.kubernetes.io/instance"] = ab.Name
	labels["app.kubernetes.io/name"] = ab.Name
	labels["atro.xyz/app-bundle"] = ab.Name

	return labels
}

func StringMapsMatch(map1, map2 map[string]string) bool {
	// Check if the lengths are equal
	if len(map1) != len(map2) {
		return false
	}

	// Iterate through the first map
	for key, value := range map1 {
		// Check if the key exists in the second map
		if val, ok := map2[key]; !ok || val != value {
			return false
		}
	}

	return true
}
