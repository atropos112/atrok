package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppBundleBaseSpec defines the desired state of AppBundleBase
type AppBundleBaseSpec struct {
	Base           *string                      `json:"base,omitempty"`
	Image          *AppBundleBaseImage          `json:"image,omitempty"`
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

type AppBundleBaseImage struct {
	Repository *string `json:"repository,omitempty"`
	Tag        *string `json:"tag,omitempty"`
	PullPolicy *string `json:"pullPolicy,omitempty"`
}

type AppBundleBaseRouteIngress struct {
	Domain string `json:"domain,omitempty"`
	Auth   bool   `json:"auth,omitempty"`
}

type AppBundleBaseVolume struct {
	Name          *string                  `json:"name,omitempty"`
	Path          *string                  `json:"path,omitempty"`
	Size          *string                  `json:"size,omitempty"`
	StorageClass  *string                  `json:"storageClass,omitempty"`
	ExistingClaim *string                  `json:"existingClaim,omitempty"`
	Longhorn      *AppBundleVolumeLonghorn `json:"longhorn,omitempty"`
}

type AppBundleBaseVolumeLonghornBackup struct {
	Frequency string `json:"frequency,omitempty"`
	Retain    int    `json:"retain,omitempty"`
}

// AppBundleBaseStatus defines the observed state of AppBundleBase
type AppBundleBaseStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

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
