package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasFinalizer checks if a Resource has the given finalizer.
func HasFinalizer(finalizer string, resource metav1.Object) bool {
	current := resource.GetFinalizers()
	for _, f := range current {
		if f == finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer adds a finalizer to a Resource to ensure cleanup is performed.
func AddFinalizer(finalizer string, resource metav1.Object) (changed bool) {
	if !HasFinalizer(finalizer, resource) {
		finalizers := resource.GetFinalizers()

		if finalizers == nil {
			finalizers = []string{finalizer}
		} else {
			finalizers = append(finalizers, finalizer)
		}

		resource.SetFinalizers(finalizers)
		return true
	}
	return false
}

// RemoveFinalizer removes the given finalizer from a Resource.
func RemoveFinalizer(finalizer string, resource metav1.Object) (changed bool) {
	finalizers := resource.GetFinalizers()
	for i, f := range finalizers {
		if f == finalizer {
			// Remove this element from the slice
			finalizers = append(finalizers[:i], finalizers[i+1:]...)
			changed = true

			if len(finalizers) > 0 {
				resource.SetFinalizers(finalizers)
			} else {
				resource.SetFinalizers(nil)
			}
		}
	}
	return
}
