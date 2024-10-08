package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
)

// CreateExpectedPVC creates the expected PVC in order to be compared to an already existing PVC if one exists, reconcille if doesn't.
func CreateExpectedPVC(ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, volumeName string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:            volumeName,
		Namespace:       ab.Namespace,
		OwnerReferences: []metav1.OwnerReference{ab.OwnerReference()},
	}}

	pvc.Spec = corev1.PersistentVolumeClaimSpec{
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		StorageClassName: volume.StorageClass,
		Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(*volume.Size)}},
	}
	pvc.ObjectMeta.Labels = GetPVCLabels(ab, volume, pvc)

	return pvc, nil
}

func GetPVCLabels(ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, pvc *corev1.PersistentVolumeClaim) map[string]string {
	labels := SetDefaultAppBundleLabels(ab, nil)
	shouldBackup := ab.Spec.Backup != nil && !(volume.Backup != nil && !*volume.Backup)
	if shouldBackup {
		reccuringJobName := fmt.Sprintf("%s-%s", ab.Name, ab.Namespace)
		jobSpecificKey := fmt.Sprintf("recurring-job.longhorn.io/%s", reccuringJobName)
		jobGenericKey := "recurring-job.longhorn.io/source"
		defaultGroupKey := "recurring-job-group.longhorn.io/default"

		labels[jobSpecificKey] = "enabled"
		labels[jobGenericKey] = "enabled"
		labels[defaultGroupKey] = "enabled"
	}
	return labels
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
		volumeName := ab.Name + "-" + key
		// IF an Existing PVC then maybe attach labels and re-upsert
		if volume.ExistingClaim != nil {
			if err := r.ReconcileExistingPVC(ctx, ab, &volume); err != nil {
				return err
			}
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
		if err := r.ReconcilePVC(ctx, ab, &volume, volumeName); err != nil {
			return err
		}
	}

	// LONGHORN backup plugin reconciliation
	if ab.Spec.Backup != nil {
		if err := r.ReconcileRecurringBackupJob(ctx, ab); err != nil {
			return err
		}
	}

	return nil
}

func (r *AppBundleReconciler) ReconcileExistingPVC(ctx context.Context, ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume) error {
	// GET CURRENT PVC
	currentPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      *volume.ExistingClaim,
		Namespace: ab.Namespace,
	}}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentPVC), currentPVC)

	// GET EXPECTED LABELS
	expectedLabels := GetPVCLabels(ab, volume, currentPVC)
	if currentPVC.Labels != nil {
		for k, v := range currentPVC.Labels {
			expectedLabels[k] = v
		}
	}

	// IF CURRENT != EXPECTED LABEL-WISE THEN UPSERT
	if !StringMapsMatch(currentPVC.Labels, expectedLabels) {
		reason, err := FormulateDiffMessageForSpecs(currentPVC.ObjectMeta.Labels, expectedLabels)
		if err != nil {
			return err
		}

		// WARN: We re-upsert the existing PVC, upserting expectedPVC will fail as defaults are not set.
		currentPVC.ObjectMeta.Labels = expectedLabels
		return UpsertResource(ctx, r, currentPVC, reason, er, true)
	}

	return nil
}

// ReconcilePVC compares currently existing PVC (if one exists) with what is the expected PVC and upserts if they differ.
func (r *AppBundleReconciler) ReconcilePVC(ctx context.Context, ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume, volumeName string) error {
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
		reason, err := FormulateDiffMessageForSpecs(currentPVC.Spec, expectedPVC.Spec)
		if err != nil {
			return err
		}

		// Maybe labels changed because of backup label reasons
		if reason == "" {
			labelReason, err := GetDiffPaths(currentPVC.ObjectMeta.GetLabels(), expectedPVC.ObjectMeta.GetLabels())
			if err != nil {
				return err
			}
			reason = "labels changed: " + labelReason
		}

		return UpsertResource(ctx, r, expectedPVC, reason, er, true)
	}

	if !StringMapsMatch(expectedPVC.ObjectMeta.Labels, currentPVC.ObjectMeta.Labels) {
		reason, err := FormulateDiffMessageForSpecs(currentPVC.ObjectMeta.Labels, expectedPVC.ObjectMeta.Labels)
		if err != nil {
			return err
		}

		// WARN: We re-upsert the existing PVC, upserting expectedPVC will fail as defaults are not set.
		currentPVC.ObjectMeta.Labels = expectedPVC.ObjectMeta.Labels
		return UpsertResource(ctx, r, currentPVC, reason, er, true)
	}

	return nil
}
