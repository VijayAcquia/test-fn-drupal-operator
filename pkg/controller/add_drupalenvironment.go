package controller

import (
	"github.com/acquia/fn-drupal-operator/pkg/controller/drupalenvironment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, drupalenvironment.Add)
}
