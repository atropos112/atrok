package controller

import (
	"context"
	"fmt"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AppBundleReconciler) ReconcileIngress(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	l := log.FromContext(ctx)
	// LOCK the resource
	mu := getMutex("ingress", ab.Name, ab.Namespace)
	mu.Lock()

	names := []string{}
	if ab.Spec.Routes != nil {
		for _, route := range ab.Spec.Routes {
			if route.Ingress != nil {
				names = append(names, route.Name)
			}
		}
	}

	// GET all ingresses with correct labels
	ingresses := &netv1.IngressList{}
	l.Info("Listing ingresses for " + ab.Name)
	if err := r.List(ctx, ingresses, client.InNamespace(ab.Namespace), client.MatchingLabels{"appbundle": ab.Name}); err != nil {
		l.Error(err, "Unable to list ingresses for "+ab.Name)
		return err
	}

	// DELETE ingresses that are not in the list
	for _, ingress := range ingresses.Items {
		if !contains(names, ingress.Name) {
			l.Info("Deleting ingress " + ingress.Name)
			if err := r.Delete(ctx, &ingress); err != nil {
				l.Error(err, "Unable to delete ingress "+ingress.Name)
				return err
			}
		}
	}

	// No ingresses exist and clean up happened by now, just leave.
	if len(names) == 0 {
		return nil
	}

	for _, route := range ab.Spec.Routes {
		if route.Ingress == nil {
			continue
		}

		// GET the resource
		ingress := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{
			Name:            ab.Name + "-" + route.Name,
			Namespace:       ab.Namespace,
			OwnerReferences: []metav1.OwnerReference{ab.OwnerReference()},
		}}

		l.Info("Getting ingress " + ingress.Name)
		er := r.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)

		// If no annotation, add it
		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string)
		}
		if ingress.Labels == nil {
			ingress.Labels = make(map[string]string)
		}

		// REGAIN control if lost
		ingress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

		// CHECK and BUILD the resource
		ingress.Labels["appbundle"] = ab.Name
		ingress.Annotations["traefik.ingress.kubernetes.io/router.entryPoints"] = entry_point
		ingress.Annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
		ingress.Annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = cluster_issuer
		ingress.Annotations["cert-manager.io/cluster-issuer"] = cluster_issuer
		if auth_middleware != "" && *route.Ingress.Auth {
			ingress.Annotations["traefik.ingress.kubernetes.io/router.middlewares"] = auth_middleware
		}

		// BUILD the resource
		rules := []netv1.IngressRule{}
		tls := []netv1.IngressTLS{}

		// Add middleware

		path_type := netv1.PathTypePrefix

		rules = append(rules, netv1.IngressRule{
			Host: *route.Ingress.Domain,
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path:     "/", // This is a uneccessary limitation to simplify the controller
							PathType: &path_type,
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: ab.Name,
									Port: netv1.ServiceBackendPort{
										Number: int32(*route.Port),
									},
								},
							},
						},
					},
				},
			},
		})

		tls = append(tls, netv1.IngressTLS{
			Hosts:      []string{*route.Ingress.Domain},
			SecretName: fmt.Sprintf("%s-%s-%s-ingress-tls", ab.Name, route.Name, ab.Namespace),
		})

		ingress.Spec = netv1.IngressSpec{
			Rules: rules,
			TLS:   tls,
		}

		// UPSERT the resource
		if err := UpsertResource(ctx, r, ingress, er); err != nil {
			l.Error(err, "Unable to upsert ingress "+ingress.Name)
			return err
		}
	}

	mu.Unlock()

	if ab.Spec.Homepage != nil {
		l.Info("Reconciling homepage for " + ab.Name)
		if err := r.ReconcileHomePage(ctx, req, ab); err != nil {
			l.Error(err, "Unable to reconcile homepage for "+ab.Name)
			return err
		}
	}

	return nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
