package controllers

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func logErrorAndRequeue(logger logr.Logger, msg string, err error) (ctrl.Result, error) {
	logger.Error(err, msg)
	return ctrl.Result{Requeue: true}, nil
}

func reconciled() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
