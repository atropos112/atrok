package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppBundleBaseSpec defines the desired state of AppBundleBase
type AppBundleBaseSpec struct {
	Base           *string                        `json:"base,omitempty"`
	Image          *AppBundleImage                `json:"image,omitempty"`
	NodeSelector   *map[string]string             `json:"nodeSelector,omitempty"`
	UseNvidia      *bool                          `json:"useNvidia,omitempty"`
	Replicas       *int32                         `json:"replicas,omitempty"`
	Resources      *corev1.ResourceRequirements   `json:"resources,omitempty"`
	Envs           map[string]string              `json:"envs,omitempty"`
	SourcedEnvs    map[string]AppBundleSourcedEnv `json:"sourcedEnvs,omitempty"`
	ServiceType    *corev1.ServiceType            `json:"serviceType,omitempty"`
	Routes         map[string]AppBundleRoute      `json:"routes,omitempty"`
	Homepage       *AppBundleHomePage             `json:"homepage,omitempty"`
	Volumes        map[string]AppBundleVolume     `json:"volumes,omitempty"`
	Backup         *AppBundleVolumeLonghornBackup `json:"backup,omitempty"`
	Selector       *metav1.LabelSelector          `json:"selector,omitempty"`
	LivenessProbe  *corev1.Probe                  `json:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe                  `json:"readinessProbe,omitempty"`
	StartupProbe   *corev1.Probe                  `json:"startupProbe,omitempty"`
	Command        []*string                      `json:"command,omitempty"`
	Args           []*string                      `json:"args,omitempty"`
}

// AppBundleBaseStatus defines the observed state of AppBundleBase
type AppBundleBaseStatus struct {
	LastReconciliation *string `json:"lastReconciliation,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=abb,path=appbundlebases,singular=appbundlebase,scope=Cluster

// AppBundleBase is the Schema for the appbundlebases API
type AppBundleBase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppBundleBaseSpec   `json:"spec,omitempty"`
	Status AppBundleBaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppBundleBaseList contains a list of AppBundleBase
type AppBundleBaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppBundleBase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppBundleBase{}, &AppBundleBaseList{})
}
