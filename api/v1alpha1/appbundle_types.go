// Package v1alpha1 is a generic package for DTOs. No versioning for now.
package v1alpha1

import (
	"reflect"

	"dario.cat/mergo"
	"github.com/rxwycdh/rxhash"

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
	SecretStoreRef *string                        `json:"secretStoreRef,omitempty"`
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

func MergeDictValues(dst, src interface{}) (interface{}, error) {
	switch dstV := dst.(type) {
	case string:
		if dstV == "" {
			return src.(string), nil
		}
		return dstV, nil
	case AppBundleVolume:
		if err := mergo.Merge(&dstV, src.(AppBundleVolume)); err != nil {
			return nil, err
		}
		return dstV, nil
	case AppBundleConfig:
		if err := mergo.Merge(&dstV, src.(AppBundleConfig)); err != nil {
			return nil, err
		}
		return dstV, nil
	case AppBundleRoute:
		if err := mergo.Merge(&dstV, src.(AppBundleRoute)); err != nil {
			return nil, err
		}
		return dstV, nil
	case AppBundleSourcedEnv:
		if err := mergo.Merge(&dstV, src.(AppBundleSourcedEnv)); err != nil {
			return nil, err
		}
		return dstV, nil
	default:
		panic("Unsupported type, the types passed in are (dst,src) = " + reflect.TypeOf(dst).String() + " and " + reflect.TypeOf(src).String())
	}
}

// func (s AppBundleConfigs) Less(i, j int) bool {
// 	return strings.Compare(s[i].FileName, s[j].FileName) < 0
// }
//
// func (s AppBundleConfigs) Len() int {
// 	return len(s)
// }
//
// func (s AppBundleConfigs) Swap(i, j int) {
// 	s[i], s[j] = s[j], s[i]
// }

type AppBundleConfig struct {
	FileName string            `json:"fileName,omitempty"`
	Content  string            `json:"content,omitempty"`
	Existing *string           `json:"existing,omitempty"`
	Secrets  map[string]string `json:"secrets,omitempty"`
	DirPath  string            `json:"dirPath,omitempty"`
	CopyOver *bool             `json:"copyOver,omitempty"`
}

type AppBundleSourcedEnv struct {
	ExternalSecret string `json:"externalSecret,omitempty"`
	Secret         string `json:"secret,omitempty"`
	ConfigMap      string `json:"configMap,omitempty"`
	Key            string `json:"key,omitempty"`
}

type AppBundleImage struct {
	Repository *string        `json:"repository,omitempty"`
	Tag        *string        `json:"tag,omitempty"`
	PullPolicy *v1.PullPolicy `json:"pullPolicy,omitempty"`
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
	Section     *string `json:"section,omitempty"`
	Href        *string `json:"href,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Name        *string `json:"name,omitempty"`
	Groups      *string `json:"groups,omitempty"`
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
func (ab AppBundle) ID() string {
	return ab.Name + "_" + ab.Namespace
}

// rxhash.HashStruct(ab.Spec)
func (ab AppBundle) GetSpecHash() (string, error) {
	return rxhash.HashStruct(ab.Spec)
}

func (ab AppBundle) GetLastReconciliation() (bool, string) {
	if ab.Status.LastReconciliation == nil {
		return false, ""
	}

	return true, *ab.Status.LastReconciliation
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
