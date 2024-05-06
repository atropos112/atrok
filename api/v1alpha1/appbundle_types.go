// Package v1alpha1 is a generic package for DTOs. No versioning for now.
package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppBundleSpec defines the desired state of AppBundle, its the core of the AppBundle (minus metadata etc.)
type AppBundleSpec struct {
	Base           *string                        `json:"base,omitempty"`
	Image          *AppBundleImage                `json:"image,omitempty"`
	NodeSelector   *map[string]string             `json:"nodeSelector,omitempty"`
	UseNvidia      *bool                          `json:"useNvidia,omitempty"`
	Replicas       *int32                         `json:"replicas,omitempty"`
	Resources      *v1.ResourceRequirements       `json:"resources,omitempty"`
	Envs           map[string]string              `json:"envs,omitempty"`
	SourcedEnvs    map[string]AppBundleSourcedEnv `json:"sourcedEnvs,omitempty"`
	ServiceType    *v1.ServiceType                `json:"serviceType,omitempty"`
	Routes         map[string]AppBundleRoute      `json:"routes,omitempty"`
	Homepage       *AppBundleHomePage             `json:"homepage,omitempty"`
	Volumes        map[string]AppBundleVolume     `json:"volumes,omitempty"`
	Backup         *AppBundleVolumeLonghornBackup `json:"backup,omitempty"`
	Selector       *metav1.LabelSelector          `json:"selector,omitempty"`
	LivenessProbe  *v1.Probe                      `json:"livenessProbe,omitempty"`
	ReadinessProbe *v1.Probe                      `json:"readinessProbe,omitempty"`
	StartupProbe   *v1.Probe                      `json:"startupProbe,omitempty"`
	TailscaleName  *string                        `json:"tailscaleName,omitempty"`
	Command        []*string                      `json:"command,omitempty"`
	Args           []*string                      `json:"args,omitempty"`
	Configs        map[string]AppBundleConfig     `json:"configs,omitempty"`
}

type AppBundleConfig struct {
	Data      string `json:"data,omitempty"`
	MountPath string `json:"mountPath,omitempty"`
}

type AppBundleSourcedEnv struct {
	Secret    string `json:"secret,omitempty"`
	ConfigMap string `json:"configMap,omitempty"`
	Key       string `json:"key,omitempty"`
}

type AppBundleImage struct {
	Repository *string `json:"repository,omitempty"`
	Tag        *string `json:"tag,omitempty"`
	PullPolicy *string `json:"pullPolicy,omitempty"`
}

type AppBundleRoute struct {
	Port       *int                   `json:"port,omitempty"`
	TargetPort *int                   `json:"targetPort,omitempty"`
	Protocol   *v1.Protocol           `json:"protocol,omitempty"`
	Ingress    *AppBundleRouteIngress `json:"ingress,omitempty"`
}

type AppBundleRouteIngress struct {
	Domain *string `json:"domain,omitempty"`
	Auth   *bool   `json:"auth,omitempty"`
}

type AppBundleHomePage struct {
	Description *string `json:"description,omitempty"`
	Group       *string `json:"group,omitempty"`
	Href        *string `json:"href,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Name        *string `json:"name,omitempty"`
	Instance    *string `json:"instance,omitempty"`
}

type AppBundleVolume struct {
	HostPath      *string `json:"hostPath,omitempty"`
	EmptyDir      *bool   `json:"emptyDir,omitempty"`
	Path          *string `json:"path,omitempty"`
	Size          *string `json:"size,omitempty"`
	StorageClass  *string `json:"storageClass,omitempty"`
	ExistingClaim *string `json:"existingClaim,omitempty"`
	Backup        *bool   `json:"backup,omitempty"`
}

type AppBundleVolumeLonghornBackup struct {
	Frequency *string `json:"frequency,omitempty"`
	Retain    *int    `json:"retain,omitempty"`
}

// AppBundleStatus defines the observed state of AppBundle
type AppBundleStatus struct {
	LastReconciliation *string `json:"lastReconciliation,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ab,path=appbundles,singular=appbundle,scope=Namespaced

// AppBundle is the Schema for the appbundles API
type AppBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppBundleSpec   `json:"spec,omitempty"`
	Status AppBundleStatus `json:"status,omitempty"`
}

// ID returns the unique ID of the appbundle
func (ab *AppBundle) ID() string {
	return ab.Name + "_" + ab.Namespace
}

func (ab *AppBundle) OwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: ab.APIVersion,
		Kind:       ab.Kind,
		Name:       ab.Name,
		UID:        ab.UID,
	}
}

//+kubebuilder:object:root=true

// AppBundleList contains a list of AppBundle
type AppBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppBundle{}, &AppBundleList{})
}
