package controller

import (
	"github.com/banzaicloud/kafka-operator/pkg/controller/alertmanager"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, alertmanager.Add)
}
