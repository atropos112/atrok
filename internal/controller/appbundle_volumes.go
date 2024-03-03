package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

// CreateExpectedPVC creates the expected PVC in order to be compared to an already existing PVC if one exists, reconcille if doesn't.
func CreateExpectedPVC(ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, volumeName string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      volumeName,
		Namespace: ab.Namespace,
	}}

	pvc.Spec = corev1.PersistentVolumeClaimSpec{
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		StorageClassName: volume.StorageClass,
		Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(*volume.Size)}}}

	return pvc, nil
}

// ReconcileVolumes is a generic function that takes in a volume, checks if its a hostPath, emptyDir or a PVC. If hostPath or a emptyDir it just returns, if a PVC it reconciles it using ReconcilePVC function. If backup is requested it is also reconciled using ReconcileBackup function.
func (r *AppBundleReconciler) ReconcileVolumes(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("volume", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// If no volumes requested leave.
	if ab.Spec.Volumes == nil {
		return nil
	}

	// figure out what kind of volume this is
	for _, key := range getSortedKeys(ab.Spec.Volumes) {
		volume := ab.Spec.Volumes[key]
		// IF an Existing PVC then nothing to be done here
		if volume.ExistingClaim != nil {
			continue
		}

		// IF a HostPath volume then nothing to be done here
		if volume.HostPath != nil {
			continue
		}

		// If its an emptydir volume, we leave it alone
		if volume.EmptyDir != nil && *volume.EmptyDir {
			continue
		}

		// It is understood to be a PVC, we reconcile.
		volumeName := ab.Name + "-" + key
		if err := r.ReconcilePVC(ctx, req, ab, &volume, volumeName); err != nil {
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

// ReconcilePVC compares currently existing PVC (if one exists) with what is the expected PVC and upserts if they differ.
func (r *AppBundleReconciler) ReconcilePVC(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, volumeName string) error {
	// GET CURRENT PVC
	currentPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      volumeName,
		Namespace: ab.Namespace,
	}}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentPVC), currentPVC)

	// GET EXPECTED PVC
	expectedPVC, err := CreateExpectedPVC(ab, volume, volumeName)
	if err != nil {
		return err
	}

	// IF CURRENT != EXPECTED THEN UPSERT
	if !equality.Semantic.DeepDerivative(expectedPVC.Spec, currentPVC.Spec) {

		return UpsertResource(ctx, r, expectedPVC, er)
	}

	return nil
}
