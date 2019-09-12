package main

import (
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	emperrors "emperror.dev/errors"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
)

func main() {
	log := ctrl.Log.WithName("test")
	ctrl.SetLogger(zap.Logger(true))
	err := errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("hello world"), "bad", "hello", "world")
	if emperrors.As(err, &errorfactory.ResourceNotReady{}) {
		log.Error(err, "helloooo")
	}
}
