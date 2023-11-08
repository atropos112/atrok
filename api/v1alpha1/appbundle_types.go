package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppBundleSpec struct {
	Image          *AppBundleImage              `json:"image"`
	Replicas       *int32                       `json:"replicas,omitempty"`
	Resources      *corev1.ResourceRequirements `json:"resources,omitempty"`
	ServiceType    *corev1.ServiceType          `json:"serviceType,omitempty"`
	Routes         []AppBundleRoute             `json:"routes,omitempty"`
	Homepage       *AppBundleHomePage           `json:"homepage,omitempty"`
	Volumes        []AppBundleVolume            `json:"volumes,omitempty"`
	Selector       *metav1.LabelSelector        `json:"selector,omitempty"`
	LivenessProbe  *corev1.Probe                `json:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe                `json:"readinessProbe,omitempty"`
	StartupProbe   *corev1.Probe                `json:"startupProbe,omitempty"`
}

type AppBundleImage struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	PullPolicy string `json:"pullPolicy,omitempty"`
}

type AppBundleRoute struct {
	Name    string                 `json:"name"`
	Port    int                    `json:"port"`
	Ingress *AppBundleRouteIngress `json:"ingress,omitempty"`
}

type AppBundleRouteIngress struct {
	Domain string `json:"domain"`
	Auth   bool   `json:"auth"`
}

type AppBundleHomePage struct {
	Description *string `json:"description,omitempty"`
	Group       *string `json:"group,omitempty"`
	Href        *string `json:"href,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Name        *string `json:"name,omitempty"`
}

type AppBundleVolume struct {
	Name          *string                  `json:"name"`
	Path          *string                  `json:"path"`
	Size          *string                  `json:"size,omitempty"`
	StorageClass  *string                  `json:"storageClass,omitempty"`
	ExistingClaim *string                  `json:"existingClaim,omitempty"`
	Longhorn      *AppBundleVolumeLonghorn `json:"longhorn,omitempty"`
}

type AppBundleVolumeLonghorn struct {
	Backup AppBundleVolumeLonghornBackup `json:"backup,omitempty"`
}

type AppBundleVolumeLonghornBackup struct {
	Frequency string `json:"frequency"`
	Retain    int    `json:"retain"`
}

// AppBundleStatus defines the observed state of AppBundle
type AppBundleStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AppBundle is the Schema for the appbundles API
type AppBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppBundleSpec   `json:"spec,omitempty"`
	Status AppBundleStatus `json:"status,omitempty"`
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
