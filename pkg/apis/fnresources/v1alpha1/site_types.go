package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate openapi"
// to regenerate code after modifying this file.
// SEE: https://book.kubebuilder.io/reference/generating-crd.html

import (
	"strings"

	batchv1b1 "k8s.io/api/batch/v1beta1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DomainMap map[string]string
type SiteId string

var siteChildLabels = []string{
	ApplicationIdLabel,
	EnvironmentIdLabel,
	SiteIdLabel,
}

// SiteSpec defines the desired state of Site
// +k8s:openapi-gen=true
type SiteSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Domains      []string    `json:"domains"`
	Environment  string      `json:"environment"`
	Install      InstallSpec `json:"install,omitempty"`      // +optional
	Crons        []CronSpec  `json:"crons,omitempty"`        // +optional
	Tls          bool        `json:"tls,omitempty"`          // +optional
	IngressClass string      `json:"ingressClass,omitempty"` // +optional
	CertIssuer   string      `json:"certIssuer,omitempty"`   // +optional
}

// Information to install the site
// +k8s:openapi-gen=true
type InstallSpec struct {
	InstallProfile string `json:"installProfile"`
	AdminUsername  string `json:"adminUsername"`
	AdminEmail     string `json:"adminEmail"`
}

// Crons to run on the site
// +k8s:openapi-gen=true
type CronSpec struct {
	// FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit,omitempty"`     // +optional
	// SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"` // +optional
	// StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds,omitempty"`    // +optional
	Suspend bool `json:"suspend,omitempty"` // +optional

	ConcurrencyPolicy batchv1b1.ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"` // +optional

	Name     string   `json:"name"`
	Command  []string `json:"command"`
	Schedule string   `json:"schedule"`
}

// SiteStatus defines the observed state of Site
// +k8s:openapi-gen=true
type SiteStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Site is the Schema for the sites API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Site struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SiteSpec   `json:"spec,omitempty"`
	Status SiteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteList contains a list of Site
type SiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Site `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Site{}, &SiteList{})
}

func (s Site) Id() SiteId {
	return SiteId(s.GetLabels()[SiteIdLabel])
}

func (s *Site) SetId(value string) {
	if s.GetLabels() == nil {
		s.SetLabels(map[string]string{})
	}
	s.ObjectMeta.Labels[SiteIdLabel] = value
}

func (s Site) ChildLabels() map[string]string {
	siteLabels := s.GetLabels()
	if siteLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(siteChildLabels))
	for _, val := range siteChildLabels {
		ls[val] = siteLabels[val]
	}

	return ls
}

func sanitize(old string) string {
	old = strings.ReplaceAll(old, `-`, ``)
	old = strings.ReplaceAll(old, `'`, ``)
	old = strings.ReplaceAll(old, `"`, ``)
	old = strings.ReplaceAll(old, `.`, ``)
	return old
}

// The return value of this function is used directly in SQL statements.  It
// must return a string that is safe for this purpose.
func (s Site) DatabaseName() string {
	return sanitize(s.Name)
}

// The return value of this function is used directly in SQL statements.  It
// must return a string that is safe for this purpose.
func (s Site) DatabaseUser() string {
	return sanitize(s.Name)
}

func (s *Site) DomainMap() DomainMap {
	m := make(DomainMap, len(s.Spec.Domains))
	for _, domain := range s.Spec.Domains {
		m[domain] = s.DatabaseName()
	}
	return m
}

func (s *Site) IngressRules() []extv1b1.IngressRule {
	value := extv1b1.IngressRuleValue{
		HTTP: &extv1b1.HTTPIngressRuleValue{
			Paths: []extv1b1.HTTPIngressPath{
				{
					Path: "/",
					Backend: extv1b1.IngressBackend{
						ServiceName: "drupal",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
		},
	}

	rules := make([]extv1b1.IngressRule, len(s.Spec.Domains))
	for i, host := range s.Spec.Domains {
		rules[i] = extv1b1.IngressRule{
			Host:             host,
			IngressRuleValue: value,
		}
	}

	return rules
}

func (s *Site) IngressTLS() []extv1b1.IngressTLS {
	if s.Spec.Tls {
		return []extv1b1.IngressTLS{{
			Hosts:      s.Spec.Domains,
			SecretName: s.Name + "-tls-secret",
		}}
	}
	return nil
}

// Returns site ingress class for kubernetes.io/ingress.class ingress annotation
func (s *Site) IngressClass() string {
	def := "nginx" //Defaults to nginx
	ic := s.Spec.IngressClass
	if ic != "" {
		return ic
	}
	return def
}

// Returns cert-manager issuer for certmanager.k8s.io/cluster-issuer ingress annotation
func (s *Site) IngressCertIssuer() string {
	def := "letsencrypt-staging" //Defaults to letsencrypt-staging
	cmi := s.Spec.CertIssuer
	if cmi != "" {
		return cmi
	}
	return def
}
