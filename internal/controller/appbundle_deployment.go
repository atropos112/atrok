package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	equality "k8s.io/apimachinery/pkg/api/equality"
)

// CreateExpectedDeployment creates expected deployment from appbundle
func CreateExpectedDeployment(ab *atroxyzv1alpha1.AppBundle) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}

	// Ports
	var ports []corev1.ContainerPort
	for _, key := range getSortedKeys(ab.Spec.Routes) {
		route := ab.Spec.Routes[key]
		ports = append(ports, corev1.ContainerPort{Name: key, ContainerPort: int32(*route.Port), Protocol: "TCP"})
	}

	// Volume Mounts
	var volumeMounts []corev1.VolumeMount
	volumeKeys := getSortedKeys(ab.Spec.Volumes)
	for _, key := range volumeKeys {
		volumeName := key
		if ab.Spec.Volumes[key].ExistingClaim != nil {
			volumeName = *ab.Spec.Volumes[key].ExistingClaim
		}

		volume := ab.Spec.Volumes[key]
		if volume.Path == nil {
			return nil, fmt.Errorf("volume %s has no path", key)
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volumeName, MountPath: *volume.Path})
	}

	// Volumes
	var volumes []corev1.Volume
	for _, key := range volumeKeys {
		// If PVC we control then we get name from volume name, for hostPath we do the same.
		name := key
		volName := ab.Name + "-" + key
		volume := ab.Spec.Volumes[key]

		// If existing PVC then we get name from existing claim
		if volume.ExistingClaim != nil {
			name = *volume.ExistingClaim
			volName = *volume.ExistingClaim
		}

		if volume.HostPath != nil {
			pathType := corev1.HostPathDirectoryOrCreate
			hostPath := corev1.HostPathVolumeSource{
				Path: *volume.HostPath,
				Type: &pathType,
			}
			volumes = append(volumes, corev1.Volume{
				Name:         name,
				VolumeSource: corev1.VolumeSource{HostPath: &hostPath},
			})
		} else if volume.EmptyDir != nil && *volume.EmptyDir {
			volumes = append(volumes, corev1.Volume{
				Name:         name,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})
		} else {
			volumes = append(volumes, corev1.Volume{
				Name:         name,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: volName}},
			})
		}
	}

	if ab.Spec.Configs != nil {
		configMapKeys := getSortedKeys(ab.Spec.Configs)
		configMapItems := make([]corev1.KeyToPath, 0)

		for _, key := range configMapKeys {
			configMapItems = append(configMapItems, corev1.KeyToPath{Key: key, Path: ab.Spec.Configs[key].MountPath})
		}

		volumes = append(volumes, corev1.Volume{
			Name: "configMap",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ab.Name},
					Items:                configMapItems,
				},
			},
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
	env := []corev1.EnvVar{}

	// Have to sort keys otherwise get infinite loop of updating
	if ab.Spec.Envs != nil {
		sortedKeys := getSortedKeys(ab.Spec.Envs)
		// Iterate through sorted keys
		for _, key := range sortedKeys {
			env = append(env, corev1.EnvVar{Name: key, Value: ab.Spec.Envs[key]})
		}
	}

	if ab.Spec.SourcedEnvs != nil {
		sortedKeys := getSortedKeys(ab.Spec.SourcedEnvs)

		// Iterate through sorted keys
		for _, key := range sortedKeys {
			envVarSource := corev1.EnvVarSource{}
			if ab.Spec.SourcedEnvs[key].Secret != "" {
				envVarSource.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ab.Spec.SourcedEnvs[key].Secret},
					Key:                  ab.Spec.SourcedEnvs[key].Key,
				}
			} else if ab.Spec.SourcedEnvs[key].ConfigMap != "" {
				envVarSource.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ab.Spec.SourcedEnvs[key].ConfigMap},
					Key:                  ab.Spec.SourcedEnvs[key].Key,
				}
			} else {
				return nil, fmt.Errorf("SourcedEnv %s has neither Secret nor ConfigMap", key)
			}

			env = append(env, corev1.EnvVar{Name: key, ValueFrom: &envVarSource})
		}
	}
	container := corev1.Container{
		Name:           ab.Name,
		Image:          fmt.Sprintf("%s:%s", repository, tag),
		Resources:      resources,
		Ports:          ports,
		Env:            env,
		VolumeMounts:   volumeMounts,
		LivenessProbe:  ab.Spec.LivenessProbe,
		ReadinessProbe: ab.Spec.ReadinessProbe,
		StartupProbe:   ab.Spec.StartupProbe,
	}

	if ab.Spec.Command != nil {
		container.Command = []string{}

		for _, command := range ab.Spec.Command {
			container.Command = append(container.Command, *command)
		}
	}

	if ab.Spec.Args != nil {
		container.Args = []string{}

		for _, arg := range ab.Spec.Args {
			container.Args = append(container.Args, *arg)
		}
	}

	if ab.Spec.UseNvidia != nil && *ab.Spec.UseNvidia {
		envs := []corev1.EnvVar{}
		if container.Env != nil {
			envs = container.Env
		}
		foundNvidiaEnv := false

		for _, env := range container.Env {
			if env.Name == "NVIDIA_VISIBLE_DEVICES" {
				foundNvidiaEnv = true
			}
		}
		if !foundNvidiaEnv {
			envs = append(envs, corev1.EnvVar{Name: "NVIDIA_VISIBLE_DEVICES", Value: "all"})
		}
		container.Env = envs
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
				Containers:       []corev1.Container{container},
			},
		},
	}

	if ab.Spec.NodeSelector != nil {
		deployment.Spec.Template.Spec.NodeSelector = *ab.Spec.NodeSelector
	}

	if ab.Spec.UseNvidia != nil && *ab.Spec.UseNvidia {
		tolerations := []corev1.Toleration{}
		if deployment.Spec.Template.Spec.Tolerations != nil {
			tolerations = deployment.Spec.Template.Spec.Tolerations
		}
		foundNvidiaToleration := false
		for _, toleration := range tolerations {
			if toleration.Key == "nvidia.com/gpu" {
				foundNvidiaToleration = true
			}
		}
		if !foundNvidiaToleration {
			tolerations = append(tolerations, corev1.Toleration{Key: "nvidia.com/gpu", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule})
		}

		deployment.Spec.Template.Spec.Tolerations = tolerations

		nvidiaRuntimeClass := "nvidia"
		deployment.Spec.Template.Spec.RuntimeClassName = &nvidiaRuntimeClass
	}

	return deployment, nil
}

// ReconcileDeployment checks currently existing deployment with the expected deployment and updates it if necessary. If no deployment exists, it creates one.
func (r *AppBundleReconciler) ReconcileDeployment(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK APPBUNDLE DEPLOYMENT MUTEX
	mu := getMutex("deployment", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET CURRENT DEPLOYMENT
	currentDeployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentDeployment), currentDeployment)

	// GET EXPECTED DEPLOYMENT
	expectedDeployment, err := CreateExpectedDeployment(ab)
	if err != nil {
		return err
	}

	// IF CURRENT != EXPECTED THEN UPSERT
	if !equality.Semantic.DeepDerivative(expectedDeployment.Spec, currentDeployment.Spec) {
		reason, err := FormulateDiffMessageForSpecs(currentDeployment.Spec, expectedDeployment.Spec)
		if err != nil {
			return err
		}

		if err := UpsertResource(ctx, r, expectedDeployment, reason, er); err != nil {
			return err
		}
	}

	return nil
}
