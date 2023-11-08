package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Basic AppBundle reconciling ingress route with single route that has ingress route", func() {
	It("Should make a simple ingress route", func() {
		By("Creating a new AppBundle and reconciling ingress route.")
		// SETUP
		ctx := context.Background()
		ab := GetBasicAppBundle()
		rec := &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// ADD ROUTE
		route := atroxyzv1alpha1.AppBundleRoute{Name: "test", Port: 80, Ingress: &atroxyzv1alpha1.AppBundleRouteIngress{Domain: "test.com", Auth: false}}
		ab.Spec.Routes = []atroxyzv1alpha1.AppBundleRoute{route}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		ApplyTypeMetaToAppBundleForTesting(ab)
		Expect(er).NotTo(HaveOccurred())

		// RECONCILE SERVICE
		fake_req := ctrl.Request{NamespacedName: client.ObjectKey{Name: ab.Name, Namespace: ab.Namespace}}
		err := rec.ReconcileIngressRoute(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())
	})
})
