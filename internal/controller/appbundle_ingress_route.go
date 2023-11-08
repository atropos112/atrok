package controller

import (
	"context"
	"fmt"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	traefikio "github.com/atropos112/atrok.git/external_apis/traefikio/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *AppBundleReconciler) ReconcileIngressRoute(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("ingress_route", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	ingress_route := &traefikio.IngressRoute{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(ingress_route), ingress_route)

	// REGAIN control if lost
	ingress_route.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	// CHECK and BUILD the resource

	// If no routes, but ingress exists, delete it
	if ab.Spec.Routes == nil && er == nil {

		if err := r.Delete(ctx, ingress_route); err != nil {
			return err
		}
		return nil
		// If routes exist and ingress exists, update it
	} else if ab.Spec.Routes != nil {

		// BUILD the resource
		routes := []traefikio.Route{}
		for _, route := range ab.Spec.Routes {
			middlewares := []traefikio.MiddlewareRef{}
			if route.Ingress.Auth {
				middlewares = append(middlewares, auth_middleware)
			}

			service := traefikio.LoadBalancerSpec{Name: ab.Name, Port: intstr.IntOrString{IntVal: int32(route.Port)}}
			routes = append(routes, traefikio.Route{
				Match:       fmt.Sprintf("Host(`%s`)", route.Ingress.Domain),
				Kind:        "Rule",
				Services:    []traefikio.Service{{LoadBalancerSpec: service}},
				Middlewares: middlewares,
			})
		}

		ingress_route.Spec = traefikio.IngressRouteSpec{
			EntryPoints: entry_points,
			Routes:      routes,
			TLS:         &traefikio.TLS{SecretName: fmt.Sprintf("%s-%s", ab.Name, ab.Namespace)},
		}

		// UPSERT the resource
		if err := UpsertResource(ctx, r, ingress_route, er); err != nil {
			return err
		}

		if ab.Spec.Homepage != nil {
			if err := r.ReconcileHomePage(ctx, req, ab); err != nil {
				return err
			}
		}

	}

	return nil
}
