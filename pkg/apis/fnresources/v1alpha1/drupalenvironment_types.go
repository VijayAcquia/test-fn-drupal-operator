package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate openapi"
// to regenerate code after modifying this file.
// SEE: https://book.kubebuilder.io/reference/generating-crd.html

import (
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EnvironmentId string

var envChildLabels = []string{
	ApplicationIdLabel,
	EnvironmentIdLabel,
}

// DrupalEnvironmentSpec defines the desired state of DrupalEnvironment
// +k8s:openapi-gen=true
type DrupalEnvironmentSpec struct {
	Application string `json:"application"`
	Production  bool   `json:"production"`
	EFSID       string `json:"efsid"`
	GitRef      string `json:"gitRef"`

	Drupal   SpecDrupal   `json:"drupal"`
	Apache   SpecApache   `json:"apache"`
	Phpfpm   SpecPhpFpm   `json:"phpfpm"`
	ProxySQL SpecProxySQL `json:"proxySQL"`
}

// SpecDrupal represents drupalenvironment.spec.drupal
type SpecDrupal struct {
	Tag                            string        `json:"tag"`
	PullPolicy                     v1.PullPolicy `json:"pullPolicy"`
	MinReplicas                    int32         `json:"minReplicas"`
	MaxReplicas                    int32         `json:"maxReplicas"`
	TargetCPUUtilizationPercentage *int32        `json:"targetCPUUtilizationPercentage,omitempty"`

	Liveness  HTTPProbe `json:"livenessProbe"`
	Readiness HTTPProbe `json:"readinessProbe"`
}

// SpecApache represents drupalenvironment.spec.apache
type SpecApache struct {
	Tag     string    `json:"tag"`
	WebRoot string    `json:"webRoot"`
	Cpu     Resources `json:"cpu"`
	Memory  Resources `json:"memory"`
}

// SpecPhpFpm represents drupalenvironment.spec.phpfpm
type SpecPhpFpm struct {
	Tag                   string    `json:"tag"`
	Procs                 int32     `json:"procs"`
	ProcMemoryLimitMiB    int32     `json:"procMemoryLimitMiB"`
	OpcacheMemoryLimitMiB int32     `json:"opcacheMemoryLimitMiB"`
	ApcMemoryLimitMiB     int32     `json:"apcMemoryLimitMiB"`
	Cpu                   Resources `json:"cpu"`
}

type SpecProxySQL struct {
	Replicas int32     `json:"replicas"`
	Cpu      Resources `json:"cpu"`
	Memory   Resources `json:"memory"`
	Tag      string    `json:"tag"`
}

// Resources specifies container resource requests and limits
type Resources struct {
	Request resource.Quantity `json:"request"`
	Limit   resource.Quantity `json:"limit"`
}

// HTTPProbe specifies a container's HTTP liveness/readiness probe
type HTTPProbe struct {
	Enabled          bool   `json:"enabled"`
	HTTPPath         string `json:"httpPath"`
	TimeoutSeconds   int32  `json:"timeoutSeconds"`
	FailureThreshold int32  `json:"failureThreshold"`
	SuccessThreshold int32  `json:"successThreshold"`
	PeriodSeconds    int32  `json:"periodSeconds"`
}

// DrupalEnvironmentStatus defines the observed state of DrupalEnvironment
// +k8s:openapi-gen=true
type DrupalEnvironmentStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DrupalEnvironment is the Schema for the drupalenvironments API
// +kubebuilder:resource:shortName=drenv;drenvs
// +k8s:openapi-gen=true
type DrupalEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DrupalEnvironmentSpec   `json:"spec,omitempty"`
	Status DrupalEnvironmentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DrupalEnvironmentList contains a list of DrupalEnvironment
type DrupalEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DrupalEnvironment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DrupalEnvironment{}, &DrupalEnvironmentList{})
}

func (e DrupalEnvironment) Id() EnvironmentId {
	return EnvironmentId(e.GetLabels()[EnvironmentIdLabel])
}

func (e *DrupalEnvironment) SetId(value string) {
	if e.GetLabels() == nil {
		e.SetLabels(map[string]string{})
	}
	e.ObjectMeta.Labels[EnvironmentIdLabel] = value
}

func (e DrupalEnvironment) ChildLabels() map[string]string {
	envLabels := e.GetLabels()
	if envLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(envChildLabels))
	for _, val := range envChildLabels {
		ls[val] = envLabels[val]
	}

	return ls
}
