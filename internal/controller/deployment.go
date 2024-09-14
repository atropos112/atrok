package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
	equality "k8s.io/apimachinery/pkg/api/equality"
)

// CreateExpectedDeployment creates expected deployment from appbundle
func CreateExpectedDeployment(ab *atroxyzv1alpha1.AppBundle) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}

	// Metadata
	deployment.ObjectMeta.Annotations = ab.GetAnnotations()
	if deployment.ObjectMeta.Annotations == nil {
		deployment.ObjectMeta.Annotations = make(map[string]string)
	}

	labels := SetDefaultAppBundleLabels(ab, nil)

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
	initContainers := make([]corev1.Container, 0)

	// Attach Configs
	if ab.Spec.Configs != nil {
		configs := ab.Spec.Configs
		for _, key := range getSortedKeys(configs) {
			config := configs[key]
			volumeName := "cm-" + key

			volumeSource := corev1.VolumeSource{}

			if config.Existing != nil {
				volumeSource = corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: *config.Existing},
						Items:                []corev1.KeyToPath{{Key: key, Path: config.FileName}},
					},
				}
			} else {
				if len(config.Secrets) == 0 {
					volumeSource = corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ab.Name},
							Items:                []corev1.KeyToPath{{Key: key, Path: config.FileName}},
						},
					}
				} else {
					volumeSource = corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: ab.Name,
							Items:      []corev1.KeyToPath{{Key: "cfg" + key, Path: config.FileName}},
						},
					}
					volumeName = "sec-" + key
				}
			}

			volumes = append(volumes, corev1.Volume{
				Name:         volumeName,
				VolumeSource: volumeSource,
			})
			mountPath := config.DirPath + "/" + config.FileName

			if config.CopyOver != nil && *config.CopyOver {
				tempMountPath := "/atrok" + config.DirPath + "/" + config.FileName
				initVolumeMounts := append(volumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: tempMountPath,
					SubPath:   config.FileName,
					ReadOnly:  true,
				},
				)

				initContainers = append(initContainers, corev1.Container{
					Name:  "copy-over-" + key,
					Image: "busybox:stable",
					Command: []string{
						"sh", "-c", // Use a shell to run multiple commands
						"cp " + tempMountPath + " " + mountPath + " && chmod 777 " + mountPath,
					},
					VolumeMounts: initVolumeMounts,
				})
			} else {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: mountPath,
					SubPath:   config.FileName,
					ReadOnly:  true,
				})
			}
		}
	}

	// busybox:stable

	// Small bits
	revHistLimit := int32(3)

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
			} else if ab.Spec.SourcedEnvs[key].ExternalSecret != "" {
				envVarSource.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ab.Name},
					Key:                  "env" + key,
				}
			} else {
				return nil, fmt.Errorf("SourcedEnv %s has neither Secret nor ConfigMap", key)
			}

			env = append(env, corev1.EnvVar{Name: key, ValueFrom: &envVarSource})
		}
	}
	imagePullPolicy := corev1.PullAlways
	if ab.Spec.Image.PullPolicy != nil {
		imagePullPolicy = *ab.Spec.Image.PullPolicy
	}

	container := corev1.Container{
		Name:            ab.Name,
		Image:           fmt.Sprintf("%s:%s", repository, tag),
		ImagePullPolicy: imagePullPolicy,
		Resources:       resources,
		Ports:           ports,
		Env:             env,
		VolumeMounts:    volumeMounts,
		LivenessProbe:   ab.Spec.LivenessProbe,
		ReadinessProbe:  ab.Spec.ReadinessProbe,
		StartupProbe:    ab.Spec.StartupProbe,
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

	matchLabels := map[string]string{AppBundleSelector: ab.Name}
	labelSelector := metav1.LabelSelector{MatchLabels: matchLabels}
	affinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      AppBundleSelector,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{ab.Name},
							},
						},
					},
				},
			},
		},
	}

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             ab.Spec.Replicas,
		RevisionHistoryLimit: &revHistLimit,
		Strategy:             appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
		Selector:             &labelSelector,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: corev1.PodSpec{
				Volumes:          volumes,
				ImagePullSecrets: image_pull_secrets,
				InitContainers:   initContainers,
				Affinity:         affinity,
				Containers:       []corev1.Container{container},
			},
		},
	}
	deployment.ObjectMeta.Labels = labels

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
		return UpsertResource(ctx, r, expectedDeployment, reason, er, false)
	}

	if !StringMapsMatch(expectedDeployment.ObjectMeta.Labels, currentDeployment.ObjectMeta.Labels) {
		reason, err := FormulateDiffMessageForSpecs(currentDeployment.ObjectMeta.Labels, expectedDeployment.ObjectMeta.Labels)
		if err != nil {
			return err
		}
		return UpsertResource(ctx, r, expectedDeployment, reason, er, false)
	}

	return nil
}
