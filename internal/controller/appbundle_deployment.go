package controller

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	rxhash "github.com/rxwycdh/rxhash"
)

func (r *AppBundleReconciler) ReconcileDeployment(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("deployment", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
	hashBeforeChanges, err := rxhash.HashStruct(deployment.Spec)
	if err != nil {
		return err
	}

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
		// If PVC we control then we get name from volume name, for hostPath we do the same.
		name := volume.Name

		// If existing PVC then we get name from existing claim
		if volume.ExistingClaim != nil {
			name = *volume.ExistingClaim
		}

		if volume.HostPath != nil {
			pathType := corev1.HostPathDirectoryOrCreate
			hostPath := corev1.HostPathVolumeSource{
				Path: *volume.HostPath,
				Type: &pathType,
			}
			volumes = append(volumes, corev1.Volume{
				Name:         volume.Name,
				VolumeSource: corev1.VolumeSource{HostPath: &hostPath},
			})
		} else {
			volumes = append(volumes, corev1.Volume{
				Name:         volume.Name,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}},
			})
		}
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
	env := []corev1.EnvVar{}

	// Have to sort keys otherwise get infinite loop of updating
	if ab.Spec.Envs != nil {
		// Collect keys and sort them
		var keys []string
		for key := range ab.Spec.Envs {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Iterate through sorted keys
		for _, key := range keys {
			env = append(env, corev1.EnvVar{Name: key, Value: ab.Spec.Envs[key]})
		}
	}

	container := corev1.Container{
		Name:           ab.Name,
		Image:          fmt.Sprintf("%s:%s", repository, tag),
		Resources:      resources,
		Ports:          ports,
		Env:            env,
		VolumeMounts:   volume_mounts,
		LivenessProbe:  ab.Spec.LivenessProbe,
		ReadinessProbe: ab.Spec.ReadinessProbe,
		StartupProbe:   ab.Spec.StartupProbe,
	}

	if ab.Spec.Command != nil {
		if container.Command == nil {
			container.Command = []string{}
		}

		for _, command := range ab.Spec.Command {
			container.Command = append(container.Command, *command)
		}
	}

	if ab.Spec.Args != nil {
		if container.Args == nil {
			container.Args = []string{}
		}

		for _, arg := range ab.Spec.Args {
			container.Args = append(container.Args, *arg)
		}
	}

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
				Containers:       []corev1.Container{container}}}}

	if ab.Spec.NodeSelector != nil {
		deployment.Spec.Template.Spec.NodeSelector = *ab.Spec.NodeSelector
	}

	hashAfterChanges, err := rxhash.HashStruct(deployment.Spec)
	if err != nil {
		return err
	}

	if hashBeforeChanges != hashAfterChanges {
		// UPSERT the resource
		if err := UpsertResource(ctx, r, deployment, er); err != nil {
			return err
		}
	}

	return nil
}
