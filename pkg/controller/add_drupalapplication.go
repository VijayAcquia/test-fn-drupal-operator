package controller

import (
	"github.com/acquia/fn-drupal-operator/pkg/controller/drupalapplication"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, drupalapplication.Add)
}
