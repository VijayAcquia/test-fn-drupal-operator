package customercontainer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

const (
	ECRRepoRoot           = "881217801864.dkr.ecr.us-east-1.amazonaws.com/" // TODO: pull from environment variable
	CustomerECRRepoPrefix = "customer/"
	CustomerECRRepoRoot   = ECRRepoRoot + CustomerECRRepoPrefix

	sharedFilesName = "shared-files"
)

func prefix(e *fnv1alpha1.DrupalEnvironment) string {
	return string(e.Id())
}

func ImageName(a *fnv1alpha1.DrupalApplication, e *fnv1alpha1.DrupalEnvironment) (imageName string) {
	if a.Spec.ImageRepo == "" {
		imageName = fmt.Sprintf(CustomerECRRepoRoot+"%v:%v", a.Spec.GitRepo, e.Spec.Drupal.Tag)
	} else {
		imageName = fmt.Sprintf("%v:%v", a.Spec.ImageRepo, e.Spec.Drupal.Tag)
	}
	return
}

func FilesVolume(e *fnv1alpha1.DrupalEnvironment) v1.Volume {
	return v1.Volume{
		Name: sharedFilesName,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: prefix(e) + "-files",
			},
		},
	}
}

func FilesVolumeMount(e *fnv1alpha1.DrupalEnvironment) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      sharedFilesName,
		MountPath: "/var/www/html/docroot/sites/default/files", // FIXME
		SubPath:   prefix(e) + "-drupal-files",
	}
}

func SharedVolumeMount(e *fnv1alpha1.DrupalEnvironment) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      sharedFilesName,
		MountPath: "/shared",
		SubPath:   prefix(e) + "-shared",
	}
}

func Template(a *fnv1alpha1.DrupalApplication, e *fnv1alpha1.DrupalEnvironment) v1.Container {
	return v1.Container{
		Image:           ImageName(a, e),
		ImagePullPolicy: e.Spec.Drupal.PullPolicy,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("200m"),
				v1.ResourceMemory: resource.MustParse("375Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		VolumeMounts: []v1.VolumeMount{
			FilesVolumeMount(e),
			SharedVolumeMount(e),
			{
				Name:      "php-config",
				MountPath: "/usr/local/etc/php/conf.d/",
				ReadOnly:  true,
			},
			{
				Name:      "env-config",
				MountPath: "/env-config/",
				ReadOnly:  true,
			},
		},
	}
}
