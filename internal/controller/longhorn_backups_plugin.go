package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	longhornv1beta2 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	rxhash "github.com/rxwycdh/rxhash"
)

// The recurring jobs are not cleaned up after app bundle is deleted which needs to be fixed
// GetAppBundleObjectMetaWithOwnerReference(ab).OwnerReferences[] gives a list of owner references (all things i depend on) this might be useful for that
func (r *AppBundleReconciler) ReconcileBackup(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	if err := r.ReconcileRecurringBackupJob(ctx, req, ab); err != nil {
		return err
	}
	reccuringJobName := fmt.Sprintf("%s-%s", ab.Name, ab.Namespace)

	job_specific_key := fmt.Sprintf("recurring-job.longhorn.io/%s", reccuringJobName)
	job_generic_key := "recurring-job.longhorn.io/source"

	for _, abVol := range ab.Spec.Volumes {
		volName := abVol.Name
		if abVol.ExistingClaim != nil {
			volName = *abVol.ExistingClaim
		}

		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: volName, Namespace: ab.Namespace}}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			return err
		}

		labels := make(map[string]string)
		for key, value := range pvc.GetLabels() {
			labels[key] = value
		}

		// If the key is not there we dont delete the generic key as it might be part of other backup place.
		if abVol.Backup != nil && !*abVol.Backup {
			delete(labels, job_specific_key)
		} else {
			labels[job_specific_key] = "enabled"
			labels[job_generic_key] = "enabled"
		}

		pvc.ObjectMeta.Labels = labels

		// REGAIN control if lost
		pvc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}
		if err := UpsertResource(ctx, r, pvc, nil); err != nil {
			return err
		}
	}

	return nil
}

func (r *AppBundleReconciler) ReconcileRecurringBackupJob(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	if ab.Spec.Backup == nil {
		return nil
	}

	if ab.Spec.Volumes == nil || len(ab.Spec.Volumes) == 0 {
		return nil
	}

	reccuringJobName := fmt.Sprintf("%s-%s", ab.Name, ab.Namespace)

	// GET the resource
	// Can't use app bundle as owner reference because it would be instantly GC'd as it's in a different namespace
	recurringJob := &longhornv1beta2.RecurringJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      reccuringJobName,
			Namespace: "longhorn-system",
		},
	}

	er := r.Get(ctx, client.ObjectKeyFromObject(recurringJob), recurringJob)

	if er != nil && !errors.IsNotFound(er) {
		return er
	}

	rjBeforeHash, err := rxhash.HashStruct(recurringJob.Spec)
	if err != nil {
		return err
	}

	// Recurring JOB
	recurringJob.Spec = longhornv1beta2.RecurringJobSpec{
		Name:        reccuringJobName,
		Groups:      []string{},
		Task:        longhornv1beta2.RecurringJobTypeBackup,
		Cron:        *ab.Spec.Backup.Frequency,
		Retain:      *ab.Spec.Backup.Retain,
		Concurrency: 1,
	}

	pvc := &corev1.PersistentVolumeClaim{}
	for _, abVol := range ab.Spec.Volumes {
		volName := abVol.Name
		if abVol.ExistingClaim != nil {
			volName = *abVol.ExistingClaim
		}

		// GET pvc so we can get underlying volume name
		pvc = &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: volName, Namespace: ab.Namespace}}

		if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			return err
		}

		if pvc.Status.Phase == corev1.ClaimBound {
			vol := &longhornv1beta2.Volume{ObjectMeta: metav1.ObjectMeta{Name: pvc.Spec.VolumeName, Namespace: "longhorn-system"}}
			if err := r.Get(ctx, client.ObjectKeyFromObject(vol), vol); err != nil {
				return err
			}
			recurringJob.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{APIVersion: "longhorn.io/v1beta2", Kind: "Volume", Name: vol.Name, UID: vol.UID}}
			break
		}
	}

	rjAfterHash, err := rxhash.HashStruct(recurringJob.Spec)
	if err != nil {
		return err
	}

	if rjBeforeHash == rjAfterHash {
		return nil
	}

	// UPSERT the resource
	if err := UpsertResource(ctx, r, recurringJob, er); err != nil {
		return err
	}

	return nil
}
