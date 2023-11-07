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
)

// The recurring jobs are not cleaned up after app bundle is deleted which needs to be fixed
// GetAppBundleObjectMetaWithOwnerReference(ab).OwnerReferences[] gives a list of owner references (all things i depend on) this might be useful for that
func (r *AppBundleReconciler) ReconcileBackup(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle, volume *atroxyzv1alpha1.AppBundleVolume) error {
	reccuringJobName := fmt.Sprintf("%s-%s", ab.Name, *volume.Name)
	// GET pvc so we know owner reference
	pvc_name := *volume.Name
	if volume.ExistingClaim != nil {
		pvc_name = *volume.ExistingClaim
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvc_name, Namespace: ab.Namespace}}

	if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
		return err
	}

	vol := &longhornv1beta2.Volume{ObjectMeta: metav1.ObjectMeta{Name: pvc.Spec.VolumeName, Namespace: "longhorn-system"}}
	if err := r.Get(ctx, client.ObjectKeyFromObject(vol), vol); err != nil {
		return err
	}

	// GET the resource
	// Can't use app bundle as owner reference because it would be instantly GC'd as it's in a different namespace
	recurringJob := &longhornv1beta2.RecurringJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      reccuringJobName,
			Namespace: "longhorn-system",
			// Owner ref belongs to the volume which links to PVC
			// If i delete bundle, GC collects the PVC which in turn causes the Volume to delete (if "Delete" is set instead of "Remain") which will cause this to GC as well.
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "longhorn.io/v1beta2", Kind: "Volume", Name: vol.Name, UID: vol.UID}}},
	}
	er := r.Get(ctx, client.ObjectKeyFromObject(recurringJob), recurringJob)

	if er != nil && !errors.IsNotFound(er) {
		return er
	}

	// BUILD the resource
	// Labeling to get Longhorn to pick it up
	job_specific_key := fmt.Sprintf("recurring-job.longhorn.io/%s", reccuringJobName)
	job_generic_key := "recurring-job.longhorn.io/source"

	labels := make(map[string]string)
	for key, value := range pvc.GetLabels() {
		labels[key] = value
	}
	labels[job_specific_key] = "enabled"
	labels[job_generic_key] = "enabled"
	pvc.ObjectMeta.Labels = labels

	// REGAIN control if lost
	pvc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	if err := UpsertResource(ctx, r, pvc, nil); err != nil {
		return err
	}

	recurringJob.Spec = longhornv1beta2.RecurringJobSpec{
		Name:        reccuringJobName,
		Groups:      []string{},
		Task:        longhornv1beta2.RecurringJobTypeBackup,
		Cron:        volume.Longhorn.Backup.Frequency,
		Retain:      volume.Longhorn.Backup.Retain,
		Concurrency: 1}
	if err := UpsertResource(ctx, r, recurringJob, er); err != nil {
		return err
	}

	// Just so it appears in ArgoCD also
	recurringJob.ObjectMeta = metav1.ObjectMeta{
		Name:            reccuringJobName,
		Namespace:       ab.Namespace,
		Labels:          ab.Labels,
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: pvc.Name, UID: pvc.UID}},
	}
	er = r.Get(ctx, client.ObjectKeyFromObject(recurringJob), recurringJob)
	if err := UpsertResource(ctx, r, recurringJob, er); err != nil {
		return err
	}

	return nil
}
