package controller

import (
	"context"
	"fmt"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *AppBundleReconciler) ReconcileIngress(ctx context.Context, req ctrl.Request, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("ingress", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET the resource
	ingress := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)

	// REGAIN control if lost
	ingress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ab.OwnerReference()}

	// If no annotation, add it
	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}

	// CHECK and BUILD the resource

	ingress.Annotations["traefik.ingress.kubernetes.io/router.entryPoints"] = fmt.Sprintf("%v", entry_points)
	ingress.Annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
	ingress.Annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = cluster_issuer
	ingress.Annotations["cert-manager.io/cluster-issuer"] = cluster_issuer

	no_of_ingresses := 0
	if ab.Spec.Routes != nil {
		for _, route := range ab.Spec.Routes {
			if route.Ingress != nil {
				no_of_ingresses++
			}
		}
	}

	// No ingresses exist and there is no ingress just leave.
	if no_of_ingresses == 0 && errors.IsNotFound(er) {
		return nil
	}

	// If no routes, but ingress exists, delete it
	if (ab.Spec.Routes == nil || no_of_ingresses == 0) && er == nil {

		if err := r.Delete(ctx, ingress); err != nil {
			return err
		}
		return nil
		// If routes exist and ingress exists, update it
	} else if ab.Spec.Routes != nil {
		if len(ab.Spec.Routes) > 1 {
			return fmt.Errorf("multiple routes not supported yet")
			// To support multiple routes, we need to create multiple ingresses which is a bit more complicated
		}

		// BUILD the resource
		rules := []netv1.IngressRule{}
		tls := []netv1.IngressTLS{}

		for _, route := range ab.Spec.Routes {
			// Add middleware
			// This is a bit silly because if we add it for one we add it for all, but it's fine for now
			// TODO: MAKE SEPARATE INGRESSES FOR EACH ROUTE. THIS IS A HACK
			if auth_middleware != "" && *route.Ingress.Auth {
				ingress.Annotations["traefik.ingress.kubernetes.io/router.middlewares"] = auth_middleware
			}
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
				SecretName: fmt.Sprintf("%s-%s-ingress-tls", ab.Name, ab.Namespace),
			})
		}

		ingress.Spec = netv1.IngressSpec{
			Rules: rules,
			TLS:   tls,
		}

		// UPSERT the resource
		if err := UpsertResource(ctx, r, ingress, er); err != nil {
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
