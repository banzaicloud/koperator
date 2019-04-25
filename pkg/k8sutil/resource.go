package k8sutil

import (
	"context"
	"errors"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func Reconcile(log logr.Logger, client runtimeClient.Client, desired runtime.Object) error {
	desiredType := reflect.TypeOf(desired)
	var current = desired.DeepCopyObject()
	key, err := runtimeClient.ObjectKeyFromObject(current)
	if err != nil {
		return emperror.With(err, "kind", desiredType)
	}
	log = log.WithValues("kind", desiredType, "name", key.Name)

	err = client.Get(context.TODO(), key, current)
	if err != nil && !apierrors.IsNotFound(err) {
		return emperror.WrapWith(err, "getting resource failed", "kind", desiredType, "name", key.Name)
	}
	if apierrors.IsNotFound(err) {
		if err := client.Create(context.TODO(), desired); err != nil {
			return emperror.WrapWith(err, "creating resource failed", "kind", desiredType, "name", key.Name)
		}
		log.Info("resource created")
	}
	if err == nil {
		switch desired.(type) {
		default:
			return emperror.With(errors.New("unexpected resource type"), "kind", desiredType, "name", key.Name)
		case *corev1.ConfigMap:
			cm := desired.(*corev1.ConfigMap)
			cm.ResourceVersion = current.(*corev1.ConfigMap).ResourceVersion
			desired = cm
		case *corev1.Service:
			svc := desired.(*corev1.Service)
			svc.ResourceVersion = current.(*corev1.Service).ResourceVersion
			svc.Spec.ClusterIP = current.(*corev1.Service).Spec.ClusterIP
			desired = svc
		case *corev1.Pod:
			pod := desired.(*corev1.Pod)
			desired = pod
		case *corev1.PersistentVolumeClaim:
			//TODO
			desired = current
		case *appsv1.Deployment:
			deploy := desired.(*appsv1.Deployment)
			deploy.ResourceVersion = current.(*appsv1.Deployment).ResourceVersion
			desired = deploy
		case *appsv1.StatefulSet:
			deploy := desired.(*appsv1.StatefulSet)
			deploy.ResourceVersion = current.(*appsv1.StatefulSet).ResourceVersion
			desired = deploy
		}
		if err := client.Update(context.TODO(), desired); err != nil {
			return emperror.WrapWith(err, "updating resource failed", "kind", desiredType, "name", key.Name)
		}
		log.Info("resource updated")
	}
	return nil
}
