package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	longhornv1beta2 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	rxhash "github.com/rxwycdh/rxhash"
)

func (r *AppBundleReconciler) ReconcileRecurringBackupJob(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
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
	recurringJob.Spec.Labels = SetDefaultAppBundleLabels(ab, nil)

	pvc := &corev1.PersistentVolumeClaim{}
	volumeKeys := getSortedKeys(ab.Spec.Volumes)
	for _, key := range volumeKeys {
		abVol := ab.Spec.Volumes[key]

		if abVol.HostPath != nil {
			continue
		}

		volName := ab.Name + "-" + key
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
	if err := UpsertResource(ctx, r, recurringJob, "", er); err != nil {
		return err
	}

	return nil
}
