package drupalenvironment

import (
	"context"
	"os"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/customercontainer"
)

const (
	// phpMemoryOverprovisionFactor is the ratio of "memory requested" : "memory limit" for PHP-FPM containers
	phpMemoryOverprovisionFactor = 1.0 / 3.0

	drupalRolloutName = "drupal"
	drupalServiceName = "drupal"
)

func drupalCodeMount(path string) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      "drupal-code",
		MountPath: path,
	}
}

func DomainMapSecretVolume() v1.Volume {
	return v1.Volume{
		Name: "env-config",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: "domain-map",
			},
		},
	}
}

func PhpConfigVolume() v1.Volume {
	return v1.Volume{
		Name: "php-config",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: "php-config"},
			},
		},
	}
}

func apacheContainer(env *fnv1alpha1.DrupalEnvironment) v1.Container {
	drupal := env.Spec.Drupal

	apacheContainer := v1.Container{
		Name:            "apache",
		Image:           "881217801864.dkr.ecr.us-east-1.amazonaws.com/apache/default:" + env.Spec.Apache.Tag,
		ImagePullPolicy: drupal.PullPolicy,
		Ports: []v1.ContainerPort{{
			ContainerPort: 8080,
			Name:          "http",
		}},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    env.Spec.Apache.Cpu.Request,
				v1.ResourceMemory: env.Spec.Apache.Memory.Request,
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    env.Spec.Apache.Cpu.Limit,
				v1.ResourceMemory: env.Spec.Apache.Memory.Limit,
			},
		},
		Env: []v1.EnvVar{{
			Name:  "DOCROOT",
			Value: "/var/www/html/" + env.Spec.Apache.WebRoot,
		}},
		VolumeMounts: []v1.VolumeMount{
			drupalCodeMount("/var/www"),
			customercontainer.FilesVolumeMount(env),
		},
	}

	if drupal.Liveness.Enabled {
		apacheContainer.LivenessProbe = &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   drupal.Liveness.HTTPPath,
					Port:   intstr.FromString("http"),
					Scheme: "HTTP",
				},
			},
			SuccessThreshold:    drupal.Liveness.SuccessThreshold,
			FailureThreshold:    drupal.Liveness.FailureThreshold,
			TimeoutSeconds:      drupal.Liveness.TimeoutSeconds,
			PeriodSeconds:       drupal.Liveness.PeriodSeconds,
			InitialDelaySeconds: 1,
		}
	}

	if drupal.Readiness.Enabled {
		apacheContainer.ReadinessProbe = &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   drupal.Readiness.HTTPPath,
					Port:   intstr.FromString("http"),
					Scheme: "HTTP",
				},
			},
			SuccessThreshold:    drupal.Readiness.SuccessThreshold,
			FailureThreshold:    drupal.Readiness.FailureThreshold,
			TimeoutSeconds:      drupal.Readiness.TimeoutSeconds,
			PeriodSeconds:       drupal.Readiness.PeriodSeconds,
			InitialDelaySeconds: 1,
		}
	}

	return apacheContainer
}

func (rh *requestHandler) phpFpmContainer() v1.Container {
	phpfpm := rh.env.Spec.Phpfpm
	phpMemoryLimit := int64(
		phpfpm.Procs*phpfpm.ProcMemoryLimitMiB+phpfpm.OpcacheMemoryLimitMiB+phpfpm.ApcMemoryLimitMiB) * 1024 * 1024

	phpFpmContainer := customercontainer.Template(rh.app, rh.env)
	phpFpmContainer.Name = "php-fpm"
	phpFpmContainer.Image = "881217801864.dkr.ecr.us-east-1.amazonaws.com/php-fpm/default:" + phpfpm.Tag
	phpFpmContainer.Resources = v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    phpfpm.Cpu.Request,
			v1.ResourceMemory: *resource.NewQuantity(int64(float64(phpMemoryLimit)*phpMemoryOverprovisionFactor), resource.BinarySI),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    phpfpm.Cpu.Limit,
			v1.ResourceMemory: *resource.NewQuantity(phpMemoryLimit, resource.BinarySI),
		},
	}
	phpFpmContainer.VolumeMounts = append(phpFpmContainer.VolumeMounts,
		v1.VolumeMount{
			Name:      "php-fpm-config",
			MountPath: "/usr/local/etc/php-fpm.d/",
			ReadOnly:  true,
		},
		drupalCodeMount("/var/www"),
	)

	return phpFpmContainer
}

func (rh *requestHandler) drupalRolloutSpec() rolloutsv1alpha1.RolloutSpec {
	ls := labelsForDeployment(rh.env)
	rootUser := int64(0)
	rolloutAutoPromote := true // TODO: may not want to auto-promote in a multisite configuration
	rolloutAutoPromoteDelay := int32(10)
	// userReadOnly := int32(0400)
	twoReplicas := int32(2)
	scaleDownDelay := int32(30) // see https://github.com/argoproj/argo-rollouts/issues/19#issuecomment-476329960

	drupal := rh.env.Spec.Drupal

	phpfpmConfigMap := v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{Name: "phpfpm-config"},
	}

	codeCopyContainer := v1.Container{
		Name:            "code-copy",
		Image:           customercontainer.ImageName(rh.app, rh.env),
		ImagePullPolicy: drupal.PullPolicy,
		Command: []string{
			"rsync", "--verbose", "--archive", "/var/www/html", "/drupal-code",
		},
		VolumeMounts: []v1.VolumeMount{
			drupalCodeMount("/drupal-code"),
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}

	sharedSetupContainer := v1.Container{
		Name:            "shared-setup",
		Image:           customercontainer.ImageName(rh.app, rh.env),
		ImagePullPolicy: drupal.PullPolicy,
		Command: []string{
			"/bin/sh", "-c",
			"mkdir -p /shared/php_sessions && mkdir -p /shared/tmp && chown www-data:www-data /shared/* ",
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &rootUser,
		},
		VolumeMounts: []v1.VolumeMount{
			customercontainer.SharedVolumeMount(rh.env),
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}

	// Apache
	apacheContainer := apacheContainer(rh.env)

	// PhpFpm
	phpFpmContainer := rh.phpFpmContainer()

	// Rollout spec
	spec := rolloutsv1alpha1.RolloutSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Strategy: rolloutsv1alpha1.RolloutStrategy{
			BlueGreenStrategy: &rolloutsv1alpha1.BlueGreenStrategy{
				ActiveService:         drupalServiceName,
				AutoPromotionEnabled:  &rolloutAutoPromote,
				AutoPromotionSeconds:  &rolloutAutoPromoteDelay,
				ScaleDownDelaySeconds: &scaleDownDelay,
			},
		},
		Replicas: &twoReplicas, // This field will actually be controlled by the HPA; this is just an initial value
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: ls,
			},
			Spec: v1.PodSpec{
				InitContainers: []v1.Container{
					codeCopyContainer,
					sharedSetupContainer,
				},
				Containers: []v1.Container{
					apacheContainer,
					phpFpmContainer,
				},
				NodeSelector: map[string]string{
					"function": "workers",
				},
				Volumes: []v1.Volume{
					customercontainer.FilesVolume(rh.env),
					{
						Name:         "drupal-code",
						VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
					},
					{
						Name:         "php-fpm-config",
						VolumeSource: v1.VolumeSource{ConfigMap: &phpfpmConfigMap},
					},
					PhpConfigVolume(),
					DomainMapSecretVolume(),
				},
			},
		},
	}
	return spec
}

// labelsForDeployment returns the labels for selecting the resources
// belonging to the given DrupalEnvironment CR name.
func labelsForDeployment(drupalEnv *fnv1alpha1.DrupalEnvironment) map[string]string {
	labels := drupalEnv.ChildLabels()
	labels["app"] = "drupal"
	return labels
}

func (rh *requestHandler) drupalService(name string) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromString("http"),
			}},
			Selector: labelsForDeployment(rh.env),
		},
	}
	// Set DrupalEnvironment instance as the owner and controller
	rh.associateResourceWithController(svc)
	return svc
}

func (rh *requestHandler) pv(name string) *v1.PersistentVolume {
	volumeMode := v1.PersistentVolumeFilesystem

	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.PersistentVolumeSpec{
			StorageClassName: "efs",
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			Capacity: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse("128Mi"),
			},
			VolumeMode: &volumeMode,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       "efs.csi.aws.com",
					VolumeHandle: rh.env.Spec.EFSID,
				},
			},
		},
	}
}

func (rh *requestHandler) pvc(name string) *v1.PersistentVolumeClaim {
	efs := "efs"

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &efs,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("128Mi"),
				},
			},
		},
	}
	if os.Getenv("USE_DYNAMIC_PROVISIONING") == "" {
		pvc.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: rh.env.ChildLabels(),
		}
		pvc.Spec.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadWriteMany}
	}
	// Set DrupalEnvironment instance as the owner and controller
	rh.associateResourceWithController(pvc)
	return pvc
}

func (rh *requestHandler) reconcileDrupalRollout() (requeue bool, err error) {
	r := rh.reconciler

	rollout := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drupalRolloutName,
			Namespace: rh.namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, rollout, func(o runtime.Object) error {
		rollout := o.(*rolloutsv1alpha1.Rollout)
		rollout.ObjectMeta.Labels = rh.env.ChildLabels()

		spec := rh.drupalRolloutSpec()

		if rollout.ObjectMeta.CreationTimestamp.IsZero() {
			// Create
			spec.DeepCopyInto(&rollout.Spec)
			rh.associateResourceWithController(rollout)
		} else {
			// Update
			syncDrupalRollout(rollout, spec)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled Drupal Rollout", "operation", op)
		return true, nil
	}
	return false, nil
}

// syncDrupalRollout compares an existing Rollout with a desired RolloutSpec and updates the Rollout
// if any modifiable fields are found to have changed. This needs to be done selectively and carefully,
// as many fields on a Rollout can't be changed after creation, and some fields have private members, which
// precludes easy use of the "cmp" library for comparison.
func syncDrupalRollout(rollout *rolloutsv1alpha1.Rollout, spec rolloutsv1alpha1.RolloutSpec) {
	spec.Strategy.DeepCopyInto(&rollout.Spec.Strategy)

	// Iterate through the Init Containers in the rollout and new spec, matching by name,
	// in case their order differs
	for ri := range rollout.Spec.Template.Spec.InitContainers {
		rc := &rollout.Spec.Template.Spec.InitContainers[ri]
		for _, sc := range spec.Template.Spec.InitContainers {
			if rc.Name == sc.Name {
				// Sync image-related fields
				rc.Image = sc.Image
				rc.ImagePullPolicy = sc.ImagePullPolicy

				// Sync resource requests/limits
				syncResources(&rc.Resources, &sc.Resources)
			}
		}
	}

	// Iterate through the Containers in the rollout and new spec, matching by name,
	// in case their order differs
	for ri := range rollout.Spec.Template.Spec.Containers {
		rc := &rollout.Spec.Template.Spec.Containers[ri]
		for _, sc := range spec.Template.Spec.Containers {
			if rc.Name == sc.Name {
				// Sync image-related fields
				rc.Image = sc.Image
				rc.ImagePullPolicy = sc.ImagePullPolicy

				// Sync resource requests/limits
				syncResources(&rc.Resources, &sc.Resources)

				// Sync probes
				rc.ReadinessProbe = sc.ReadinessProbe
				rc.LivenessProbe = sc.LivenessProbe
				break
			}
		}
	}
}

// syncResources compares the ResoureRequirements in rr to the desired values in d. Any differences
// are copied into rr
func syncResources(rr, d *v1.ResourceRequirements) {
	if rr.Limits.Cpu().Cmp(*d.Limits.Cpu()) != 0 || rr.Limits.Memory().Cmp(*d.Limits.Memory()) != 0 {
		d.Limits.DeepCopyInto(&rr.Limits)
	}
	if rr.Requests.Cpu().Cmp(*d.Requests.Cpu()) != 0 || rr.Requests.Memory().Cmp(*d.Requests.Memory()) != 0 {
		d.Requests.DeepCopyInto(&rr.Requests)
	}
}

func (rh *requestHandler) reconcileDrupalService() (requeue bool, err error) {
	r := rh.reconciler
	name := "drupal"

	svc := rh.drupalService(name)

	found := &v1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: rh.namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating Service", "Namespace", svc.Namespace, "Name", svc.Name)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			rh.logger.Error(err, "Failed to create Service", "Namespace", svc.Namespace, "Name", svc.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile Service", "Namespace", rh.namespace, "Name", name)
		return false, err
	}

	// TODO: update selector if child labels change

	return false, nil
}

func (rh *requestHandler) reconcilePV() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	pv := rh.pv(name)

	found := &v1.PersistentVolume{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating PV", "Namespace", pv.Namespace, "Name", pv.Name)
		err = r.client.Create(context.TODO(), pv)
		if err != nil {
			rh.logger.Error(err, "Failed to create PV", "Namespace", pv.Namespace, "Name", pv.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile PV", "Name", name)
		return false, err
	}

	// Verify that PV matches expected Spec
	// TODO: do we actually want PV to update ???
	if pvNeedsUpdate(found, pv) {
		rh.logger.Info("Updating PV", "Namespace", found.Namespace, "Name", found.Name)

		found.Spec = pv.Spec

		err = r.client.Update(context.TODO(), found)
		if err != nil {
			rh.logger.Error(err, "Failed to update PV", "Namespace", found.Namespace, "Name", found.Name)
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func (rh *requestHandler) finalizePV() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	found := &v1.PersistentVolume{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: name}, found); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if found.DeletionTimestamp == nil {
		rh.logger.Info("Deleting PV", "pv name", found.Name)
		return true, r.client.Delete(context.TODO(), found)
	}
	return false, nil
}

func (rh *requestHandler) reconcilePVC() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	pvc := rh.pvc(name)

	found := &v1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: rh.namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating PVC", "Namespace", pvc.Namespace, "Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			rh.logger.Error(err, "Failed to create PVC", "Namespace", pvc.Namespace, "Name", pvc.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile PVC", "Namespace", rh.namespace, "Name", name)
		return false, err
	}

	// PVCs can't be Updated

	return false, nil
}

func pvNeedsUpdate(found, pv *v1.PersistentVolume) bool {
	if found.Spec.PersistentVolumeSource.CSI.VolumeHandle != pv.Spec.PersistentVolumeSource.CSI.VolumeHandle {
		return true
	}

	return false
}
