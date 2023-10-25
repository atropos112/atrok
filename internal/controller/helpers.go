package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReaderWriter interface {
	client.Reader
	client.Writer
}

// UpsertLabel of an existing object
func UpsertLabelIntoResource(ctx context.Context, r ReaderWriter, key string, value string, obj client.Object, id client.ObjectKey) error {
	if err := r.Get(ctx, id, obj); err != nil {
		return err
	}

	labels := obj.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	labels[key] = value

	obj.SetLabels(labels)

	if err := r.Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func UpsertResource(ctx context.Context, r ReaderWriter, obj client.Object) error {
	l := log.FromContext(ctx)

	err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		l.Info("Creating resource.", obj.GetName(), obj)
		defer r.Create(ctx, obj)
	} else {
		l.Info("Updating resource.", obj.GetName(), obj)
		defer r.Update(ctx, obj)
	}

	return nil
}
