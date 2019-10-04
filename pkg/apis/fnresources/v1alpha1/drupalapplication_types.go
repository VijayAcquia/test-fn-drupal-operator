package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate openapi"
// to regenerate code after modifying this file.
// SEE: https://book.kubebuilder.io/reference/generating-crd.html

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ApplicationId string

var appChildLabels = []string{
	ApplicationIdLabel,
}

// DrupalApplicationSpec defines the desired state of a Drupal Application
// +k8s:openapi-gen=true
type DrupalApplicationSpec struct {
	ImageRepo string `json:"imageRepo,omitempty"` // +optional
	GitRepo   string `json:"gitRepo"`
}

// DrupalEnvironmentRef defines a reference to a DrupalEnvironment
type DrupalEnvironmentRef struct {
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	EnvironmentID string    `json:"environmentID,omitempty"` // +optional
	UID           types.UID `json:"uid"`
}

// DrupalApplicationStatus defines the observed state of a Drupal Application
// +k8s:openapi-gen=true
type DrupalApplicationStatus struct {
	NumEnvironments int32                  `json:"numEnvironments"`
	Environments    []DrupalEnvironmentRef `json:"environments,omitempty"` // +optional
}

// DrupalApplication is the Schema for the drupalapplications API
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=drapps;drapp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Envs",type="integer",JSONPath=".status.numEnvironments"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type DrupalApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DrupalApplicationSpec   `json:"spec,omitempty"`
	Status DrupalApplicationStatus `json:"status,omitempty"` // +optional
}

// DrupalApplicationList contains a list of DrupalApplication
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DrupalApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DrupalApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DrupalApplication{}, &DrupalApplicationList{})
}

func (a DrupalApplication) Id() ApplicationId {
	return ApplicationId(a.GetLabels()[ApplicationIdLabel])
}

func (a *DrupalApplication) SetId(value string) {
	if a.GetLabels() == nil {
		a.SetLabels(map[string]string{})
	}
	a.ObjectMeta.Labels[ApplicationIdLabel] = value
}

func (a DrupalApplication) ChildLabels() map[string]string {
	appLabels := a.GetLabels()
	if appLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(appChildLabels))
	for _, val := range appChildLabels {
		ls[val] = appLabels[val]
	}

	return ls
}
