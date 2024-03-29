// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CronSpec) DeepCopyInto(out *CronSpec) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CronSpec.
func (in *CronSpec) DeepCopy() *CronSpec {
	if in == nil {
		return nil
	}
	out := new(CronSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in DomainMap) DeepCopyInto(out *DomainMap) {
	{
		in := &in
		*out = make(DomainMap, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
		return
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DomainMap.
func (in DomainMap) DeepCopy() DomainMap {
	if in == nil {
		return nil
	}
	out := new(DomainMap)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalApplication) DeepCopyInto(out *DrupalApplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalApplication.
func (in *DrupalApplication) DeepCopy() *DrupalApplication {
	if in == nil {
		return nil
	}
	out := new(DrupalApplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DrupalApplication) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalApplicationList) DeepCopyInto(out *DrupalApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DrupalApplication, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalApplicationList.
func (in *DrupalApplicationList) DeepCopy() *DrupalApplicationList {
	if in == nil {
		return nil
	}
	out := new(DrupalApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DrupalApplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalApplicationSpec) DeepCopyInto(out *DrupalApplicationSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalApplicationSpec.
func (in *DrupalApplicationSpec) DeepCopy() *DrupalApplicationSpec {
	if in == nil {
		return nil
	}
	out := new(DrupalApplicationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalApplicationStatus) DeepCopyInto(out *DrupalApplicationStatus) {
	*out = *in
	if in.Environments != nil {
		in, out := &in.Environments, &out.Environments
		*out = make([]DrupalEnvironmentRef, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalApplicationStatus.
func (in *DrupalApplicationStatus) DeepCopy() *DrupalApplicationStatus {
	if in == nil {
		return nil
	}
	out := new(DrupalApplicationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalEnvironment) DeepCopyInto(out *DrupalEnvironment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalEnvironment.
func (in *DrupalEnvironment) DeepCopy() *DrupalEnvironment {
	if in == nil {
		return nil
	}
	out := new(DrupalEnvironment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DrupalEnvironment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalEnvironmentList) DeepCopyInto(out *DrupalEnvironmentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DrupalEnvironment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalEnvironmentList.
func (in *DrupalEnvironmentList) DeepCopy() *DrupalEnvironmentList {
	if in == nil {
		return nil
	}
	out := new(DrupalEnvironmentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DrupalEnvironmentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalEnvironmentRef) DeepCopyInto(out *DrupalEnvironmentRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalEnvironmentRef.
func (in *DrupalEnvironmentRef) DeepCopy() *DrupalEnvironmentRef {
	if in == nil {
		return nil
	}
	out := new(DrupalEnvironmentRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalEnvironmentSpec) DeepCopyInto(out *DrupalEnvironmentSpec) {
	*out = *in
	in.Drupal.DeepCopyInto(&out.Drupal)
	in.Apache.DeepCopyInto(&out.Apache)
	in.Phpfpm.DeepCopyInto(&out.Phpfpm)
	in.ProxySQL.DeepCopyInto(&out.ProxySQL)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalEnvironmentSpec.
func (in *DrupalEnvironmentSpec) DeepCopy() *DrupalEnvironmentSpec {
	if in == nil {
		return nil
	}
	out := new(DrupalEnvironmentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrupalEnvironmentStatus) DeepCopyInto(out *DrupalEnvironmentStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrupalEnvironmentStatus.
func (in *DrupalEnvironmentStatus) DeepCopy() *DrupalEnvironmentStatus {
	if in == nil {
		return nil
	}
	out := new(DrupalEnvironmentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPProbe) DeepCopyInto(out *HTTPProbe) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPProbe.
func (in *HTTPProbe) DeepCopy() *HTTPProbe {
	if in == nil {
		return nil
	}
	out := new(HTTPProbe)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InstallSpec) DeepCopyInto(out *InstallSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InstallSpec.
func (in *InstallSpec) DeepCopy() *InstallSpec {
	if in == nil {
		return nil
	}
	out := new(InstallSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Resources) DeepCopyInto(out *Resources) {
	*out = *in
	out.Request = in.Request.DeepCopy()
	out.Limit = in.Limit.DeepCopy()
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Resources.
func (in *Resources) DeepCopy() *Resources {
	if in == nil {
		return nil
	}
	out := new(Resources)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Site) DeepCopyInto(out *Site) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Site.
func (in *Site) DeepCopy() *Site {
	if in == nil {
		return nil
	}
	out := new(Site)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Site) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SiteList) DeepCopyInto(out *SiteList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Site, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SiteList.
func (in *SiteList) DeepCopy() *SiteList {
	if in == nil {
		return nil
	}
	out := new(SiteList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SiteList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SiteSpec) DeepCopyInto(out *SiteSpec) {
	*out = *in
	if in.Domains != nil {
		in, out := &in.Domains, &out.Domains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.Install = in.Install
	if in.Crons != nil {
		in, out := &in.Crons, &out.Crons
		*out = make([]CronSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SiteSpec.
func (in *SiteSpec) DeepCopy() *SiteSpec {
	if in == nil {
		return nil
	}
	out := new(SiteSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SiteStatus) DeepCopyInto(out *SiteStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SiteStatus.
func (in *SiteStatus) DeepCopy() *SiteStatus {
	if in == nil {
		return nil
	}
	out := new(SiteStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SpecApache) DeepCopyInto(out *SpecApache) {
	*out = *in
	in.Cpu.DeepCopyInto(&out.Cpu)
	in.Memory.DeepCopyInto(&out.Memory)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SpecApache.
func (in *SpecApache) DeepCopy() *SpecApache {
	if in == nil {
		return nil
	}
	out := new(SpecApache)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SpecDrupal) DeepCopyInto(out *SpecDrupal) {
	*out = *in
	if in.TargetCPUUtilizationPercentage != nil {
		in, out := &in.TargetCPUUtilizationPercentage, &out.TargetCPUUtilizationPercentage
		*out = new(int32)
		**out = **in
	}
	out.Liveness = in.Liveness
	out.Readiness = in.Readiness
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SpecDrupal.
func (in *SpecDrupal) DeepCopy() *SpecDrupal {
	if in == nil {
		return nil
	}
	out := new(SpecDrupal)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SpecPhpFpm) DeepCopyInto(out *SpecPhpFpm) {
	*out = *in
	in.Cpu.DeepCopyInto(&out.Cpu)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SpecPhpFpm.
func (in *SpecPhpFpm) DeepCopy() *SpecPhpFpm {
	if in == nil {
		return nil
	}
	out := new(SpecPhpFpm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SpecProxySQL) DeepCopyInto(out *SpecProxySQL) {
	*out = *in
	in.Cpu.DeepCopyInto(&out.Cpu)
	in.Memory.DeepCopyInto(&out.Memory)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SpecProxySQL.
func (in *SpecProxySQL) DeepCopy() *SpecProxySQL {
	if in == nil {
		return nil
	}
	out := new(SpecProxySQL)
	in.DeepCopyInto(out)
	return out
}
