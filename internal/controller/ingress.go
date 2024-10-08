package controller

import (
	"context"
	"fmt"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateExpectedIngress creates the expected ingress from the appbundle and the name given
func CreateExpectedIngress(ab *atroxyzv1alpha1.AppBundle, name string, route *atroxyzv1alpha1.AppBundleRoute) (*netv1.Ingress, error) {
	ingress := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{
		Name:            name,
		Namespace:       ab.Namespace,
		OwnerReferences: []metav1.OwnerReference{ab.OwnerReference()},
		Annotations:     ab.ObjectMeta.Annotations,
	}}

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
	ingress.Labels = SetDefaultAppBundleLabels(ab, ingress.Labels)
	ingress.Annotations["traefik.ingress.kubernetes.io/router.entryPoints"] = entry_point
	ingress.Annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
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
		SecretName: fmt.Sprintf("%s-%s-ingress-tls", name, ab.Namespace),
	})

	ingress.Spec = netv1.IngressSpec{
		Rules: rules,
		TLS:   tls,
	}

	// check if ingress.Name ends on "web" and if ab.Spec.Homepage is not nil
	if len(ingress.Name) > 3 && ingress.Name[len(ingress.Name)-3:] == "web" && ab.Spec.Homepage != nil {
		ingress.SetAnnotations(GetHomePageAnnotations(ingress.Annotations, ab))
	}

	return ingress, nil
}

func (r *AppBundleReconciler) ReconcileIngress(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	l := log.FromContext(ctx)

	// LOCK THE APP BUNDLE INGRESS MUTEX
	mu := getMutex("ingresses", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET NAMES OF EXPECTED INGRESSES
	names := []string{}
	if ab.Spec.Routes != nil {
		for _, key := range getSortedKeys(ab.Spec.Routes) {
			route := ab.Spec.Routes[key]
			if route.Ingress != nil {
				names = append(names, ab.Name+"-"+key)
			}
		}
	}

	// GET CURRENT INGRESSES
	ingresses := &netv1.IngressList{}
	if err := r.List(ctx, ingresses, client.InNamespace(ab.Namespace), client.MatchingLabels{AppBundleSelector: ab.Name}); err != nil {
		return err
	}

	if ab.Spec.Routes == nil && (ingresses.Items == nil || len(ingresses.Items) == 0) {
		return nil
	}

	// DELETE CURRENT INGRESSES THAT ARE NOT IN THE EXPECTED NAMES LIST
	for _, ingress := range ingresses.Items {
		if !contains(names, ingress.Name) {
			l.Info("Deleting ingress " + ingress.Name)
			if err := r.Delete(ctx, &ingress); err != nil {
				return err
			}
		}
	}

	// IF EXPECTED NUMBER OF INGRESSES IS 0 THEN RETURN
	if len(names) == 0 {
		return nil
	}

	// ITERATE OVER THE EXPECTED INGRESSES
	for _, key := range getSortedKeys(ab.Spec.Routes) {
		route := ab.Spec.Routes[key]
		if route.Ingress == nil {
			// If no ingress, continue
			continue
		}

		ingressName := ab.Name + "-" + key

		// GET THE EXPECTED INGRESS
		expectedIngress, err := CreateExpectedIngress(ab, ingressName, &route)
		if err != nil {
			return err
		}

		// GET THE CURRENT INGRESS
		currentIngress := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{
			Name:            ingressName,
			Namespace:       ab.Namespace,
			OwnerReferences: []metav1.OwnerReference{ab.OwnerReference()},
		}}
		er := r.Get(ctx, client.ObjectKeyFromObject(currentIngress), currentIngress)

		// IF CURRENT != EXPECTED THEN UPSERT
		if !equality.Semantic.DeepDerivative(expectedIngress.Spec, currentIngress.Spec) {
			reason, err := FormulateDiffMessageForSpecs(currentIngress.Spec, expectedIngress.Spec)
			if err != nil {
				return err
			}

			if err := UpsertResource(ctx, r, expectedIngress, reason, er, false); err != nil {
				return err
			}
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
