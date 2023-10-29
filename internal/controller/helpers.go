package controller

import (
	"context"
	"reflect"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReaderWriter interface {
	client.Reader
	client.Writer
}

// UpsertLabel of an existing object
func UpsertLabelIntoResource(ctx context.Context, r ReaderWriter, kv map[string]string, obj client.Object, id client.ObjectKey) error {
	if err := r.Get(ctx, id, obj); err != nil {
		return err
	}

	labels := obj.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range kv {
		labels[key] = value
	}

	obj.SetLabels(labels)

	if err := r.Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func UpsertResource(ctx context.Context, r ReaderWriter, obj client.Object) error {
	l := log.FromContext(ctx)
	objClone := obj.DeepCopyObject()

	err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if objClone == obj {
		return nil
	}

	if errors.IsNotFound(err) {
		l.Info("Creating resource.", "type", reflect.TypeOf(obj).String(), "object", obj)
		if err := r.Create(ctx, obj); err != nil {
			return err
		}
	} else {
		l.Info("Updating resource.", "type", reflect.TypeOf(obj).String(), "object", obj)
		if err := r.Update(ctx, obj); err != nil {
			return err
		}
	}

	return nil
}

func RunReconciles(
	ctx context.Context,
	r ReaderWriter,
	req ctrl.Request,
	app_bundle *atroxyzv1alpha1.AppBundle,
	reconciles ...func(context.Context, ctrl.Request, *atroxyzv1alpha1.AppBundle) error,
) error {
	errs, ctx := errgroup.WithContext(ctx)

	for _, reconcile := range reconciles {
		currentReconcile := reconcile // Capture current value of reconcile
		errs.Go(func() error { return currentReconcile(ctx, req, app_bundle) })
	}

	return errs.Wait()
}
