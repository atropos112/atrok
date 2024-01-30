package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

func (r *AppBundleReconciler) ReconcileVolumes(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("volume", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	if ab.Spec.Volumes == nil {
		return nil
	}

	volumeKeys := getSortedKeys(ab.Spec.Volumes)

	for _, key := range volumeKeys {
		volume := ab.Spec.Volumes[key]
		// IF an Existing PVC then nothing to be done here
		if volume.ExistingClaim != nil {
			// If its an existing claim, we leave it alone
			continue
		}

		// IF a HostPath volume then nothing to be done here
		if volume.HostPath != nil {
			// If its a hostpath volume, we leave it alone
			continue
		}

		if volume.EmptyDir != nil && *volume.EmptyDir {
			// If its an emptydir volume, we leave it alone
			continue
		}

		if err := r.ReconcilePVC(ctx, req, ab, &volume, key); err != nil {
			return err
		}
	}

	// LONGHORN backup plugin reconciliation
	if ab.Spec.Backup != nil {
		if err := r.ReconcileBackup(ctx, req, ab); err != nil {
			return err
		}
	}

	return nil
}

func (r *AppBundleReconciler) ReconcilePVC(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, volumeName string) error {
	// GET the resource
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      volumeName,
		Namespace: ab.Namespace,
	}}
	er := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)

	// REGAIN control if lost
	pvc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	// If not existing claim, its up to us to create and manage it
	// Can only change spec of an existing PVC
	if errors.IsNotFound(er) {
		// BUILD the resource
		pvc.Spec = corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: volume.StorageClass,
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(*volume.Size)}}}

		// UPSERT the resource
		if err := UpsertResource(ctx, r, pvc, er); err != nil {
			return err
		}
	}

	return nil
}
