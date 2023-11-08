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

var _ = Describe("Basic AppBundle reconciling service", func() {
	It("Should make no service as there are no routes", func() {
		By("Creating a new AppBundle and reconciling service")
		// SETUP
		ctx := context.Background()
		ab := GetBasicAppBundle()
		rec := &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab) // This will strip the TypeMeta but we don't need it for this scenario as no service is created with this owner ref

		// RECONCILE SERVICE
		fake_req := ctrl.Request{NamespacedName: client.ObjectKey{Name: ab.Name, Namespace: ab.Namespace}}
		err := rec.ReconcileService(ctx, fake_req, ab)

		Expect(er).NotTo(HaveOccurred())
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Basic AppBundle reconciling service with single route", func() {
	It("Should make a simple service", func() {
		By("Creating a new AppBundle and reconciling service")
		// SETUP
		ctx := context.Background()
		ab := GetBasicAppBundle()
		rec := &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// ADD ROUTE
		route := atroxyzv1alpha1.AppBundleRoute{Name: "test", Port: 80, Ingress: nil}
		ab.Spec.Routes = []atroxyzv1alpha1.AppBundleRoute{route}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		ApplyTypeMetaToAppBundleForTesting(ab)

		// RECONCILE SERVICE
		fake_req := ctrl.Request{NamespacedName: client.ObjectKey{Name: ab.Name, Namespace: ab.Namespace}}
		err := rec.ReconcileService(ctx, fake_req, ab)

		Expect(er).NotTo(HaveOccurred())
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Basic AppBundle reconciling service with multiple routes", func() {
	It("Should make a simple service", func() {
		By("Creating a new AppBundle and reconciling service")
		// SETUP
		ctx := context.Background()
		ab := GetBasicAppBundle()
		rec := &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// ADD ROUTE
		ab.Spec.Routes = []atroxyzv1alpha1.AppBundleRoute{
			{Name: "test", Port: 80, Ingress: nil},
			{Name: "test2", Port: 81, Ingress: nil},
		}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		ApplyTypeMetaToAppBundleForTesting(ab)

		// RECONCILE SERVICE
		fake_req := ctrl.Request{NamespacedName: client.ObjectKey{Name: ab.Name, Namespace: ab.Namespace}}
		err := rec.ReconcileService(ctx, fake_req, ab)

		Expect(er).NotTo(HaveOccurred())
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Basic AppBundle with a single route that has ingress route", func() {
	It("Should make a simple service and reconcille safely again", func() {
		By("Creating a new AppBundle and reconciling service")
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
		err := rec.ReconcileService(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())

		// RECONCILE SERVICE again
		err = rec.ReconcileService(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Basic AppBundle reconciling service with single route that is then removed", func() {
	It("Should make a simple service and then remove it", func() {
		By("Creating a new AppBundle and reconciling service twice, once on craete and once on change")
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
		err := rec.ReconcileService(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())

		// CHANGE APPBUNDLE
		ab.Spec.Routes = nil
		er = rec.Update(ctx, ab)
		Expect(er).NotTo(HaveOccurred())

		ApplyTypeMetaToAppBundleForTesting(ab)

		// RECONCILE SERVICE again
		err = rec.ReconcileService(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())
	})
})
