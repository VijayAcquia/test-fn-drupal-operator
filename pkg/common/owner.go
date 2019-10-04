package common

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

// IsControlledBy returns true if the given object has a controller owner reference to the given "owner"
func IsControlledBy(owner, obj metav1.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Controller != nil && *ownerRef.Controller && ownerRef.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

// syncLabelsWithPrefix copies labels with the given prefix from the "src" resource to "dest"
func syncLabelsWithPrefix(src metav1.Object, dest metav1.Object, prefix string) (changed bool) {
	labels := dest.GetLabels()

	for srcKey, srcValue := range src.GetLabels() {
		if strings.HasPrefix(srcKey, prefix) {
			destValue, ok := labels[srcKey]
			if !ok || destValue != srcValue {
				changed = true
				labels[srcKey] = srcValue
			}
		}
	}

	dest.SetLabels(labels)
	return
}

func LinkToOwner(owner metav1.Object, child metav1.Object, scheme *runtime.Scheme) (changed bool, err error) {
	changed = syncLabelsWithPrefix(owner, child, fnv1alpha1.LabelPrefix)

	if !IsControlledBy(owner, child) {
		// Set owner of this Site
		err = controllerutil.SetControllerReference(owner, child, scheme)
		if err != nil {
			return false, err
		}
		changed = true
	}

	return changed, nil
}
