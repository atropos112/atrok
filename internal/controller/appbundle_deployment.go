package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
)

func (r *AppBundleReconciler) ReconcileDeployment(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("deployment", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)

	// REGAIN control if lost
	deployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	// CHECK and BUILD the resource

	// Ports
	var ports []corev1.ContainerPort
	for _, route := range ab.Spec.Routes {
		ports = append(ports, corev1.ContainerPort{Name: route.Name, HostPort: int32(*route.Port), ContainerPort: int32(*route.Port), Protocol: "TCP"})
	}

	// Volume Mounts
	var volume_mounts []corev1.VolumeMount
	for _, volume := range ab.Spec.Volumes {
		volume_mounts = append(volume_mounts, corev1.VolumeMount{Name: volume.Name, MountPath: *volume.Path})
	}

	// Volumes
	var volumes []corev1.Volume
	for _, volume := range ab.Spec.Volumes {
		name := volume.Name
		if volume.ExistingClaim != nil {
			name = *volume.ExistingClaim
		}
		volumes = append(volumes, corev1.Volume{
			Name:         volume.Name,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}},
		})
	}

	// Small bits
	revision_history_limit := int32(3)
	labels := make(map[string]string)
	for key, value := range ab.GetLabels() {
		labels[key] = value
	}
	labels["app"] = ab.Name

	resources := corev1.ResourceRequirements{}
	if ab.Spec.Resources != nil {
		resources = *ab.Spec.Resources
	}

	repository := *ab.Spec.Image.Repository
	tag := *ab.Spec.Image.Tag
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             ab.Spec.Replicas,
		RevisionHistoryLimit: &revision_history_limit,
		Strategy:             appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
		Selector:             &metav1.LabelSelector{MatchLabels: map[string]string{"app": ab.Name}},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Volumes:          volumes,
				ImagePullSecrets: image_pull_secrets,
				Containers: []corev1.Container{
					{
						Name:           ab.Name,
						Image:          fmt.Sprintf("%s:%s", repository, tag),
						Resources:      resources,
						Ports:          ports,
						VolumeMounts:   volume_mounts,
						LivenessProbe:  ab.Spec.LivenessProbe,
						ReadinessProbe: ab.Spec.ReadinessProbe,
						StartupProbe:   ab.Spec.StartupProbe,
					}}}}}

	// UPSERT the resource
	if err := UpsertResource(ctx, r, deployment, er); err != nil {
		return err
	}

	return nil
}
