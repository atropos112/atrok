package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle with no routes reconcilling ingress route", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleReconciler
	var ctx context.Context
	var fake_req ctrl.Request

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		rec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}
		fake_req = ctrl.Request{NamespacedName: client.ObjectKey{Name: ab.Name, Namespace: ab.Namespace}}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
	})

	It("Should make no ingress route as there are no routes", func() {
		By("Reconciling ingress route using app bundle")
		err := rec.ReconcileIngress(ctx, fake_req, ab)
		Expect(err).NotTo(HaveOccurred())

		// GET the resource
		ingress := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err = rec.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	Describe("Adding a single route to AppBundle with ingress route", func() {
		Context("And updating", func() {
			BeforeEach(func() {
				//ADD ROUTE
				route_name := "test"
				port := 80
				domain := "test.com"
				auth := true

				route := atroxyzv1alpha1.AppBundleRoute{Name: route_name, Port: &port, Ingress: &atroxyzv1alpha1.AppBundleRouteIngress{Domain: &domain, Auth: &auth}}
				ab.Spec.Routes = []atroxyzv1alpha1.AppBundleRoute{route}

				// UPDATE APPBUNDLE
				er := rec.Update(ctx, ab)
				Expect(er).NotTo(HaveOccurred())
				ApplyTypeMetaToAppBundleForTesting(ab)
			})

			It("Should make a simple ingress route", func() {
				By("Reconciling ingerss route using app bundle")
				err := rec.ReconcileIngress(ctx, fake_req, ab)
				Expect(err).NotTo(HaveOccurred())

				// GET the resource
				ingress := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
				err = rec.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
				Expect(err).NotTo(HaveOccurred())

				// CHECK the resource
				//Expect(service.Spec.Ports).To(HaveLen(1))
				Expect(ingress.Spec.Rules).To(HaveLen(1))
				Expect(ingress.Spec.TLS).NotTo(BeNil())
			})

			Describe("With an ingress route", func() {
				Context("And created ingress route", func() {
					BeforeEach(func() {
						err := rec.ReconcileIngress(ctx, fake_req, ab)
						Expect(err).NotTo(HaveOccurred())
					})

					It("Should not change the ingress route", func() {
						By("Reconciling ingress route again using app bundle")

						// GET ingress
						ingressBefore := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err := rec.Get(ctx, client.ObjectKeyFromObject(ingressBefore), ingressBefore)
						Expect(err).NotTo(HaveOccurred())

						// RECONCILE ingress
						err = rec.ReconcileIngress(ctx, fake_req, ab)
						Expect(err).NotTo(HaveOccurred())

						// GET ingress
						ingressAfter := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err = rec.Get(ctx, client.ObjectKeyFromObject(ingressAfter), ingressAfter)
						Expect(err).NotTo(HaveOccurred())

						// CHECK ingress route
						Expect(ingressBefore).To(Equal(ingressAfter))
					})

					It("Should delete the ingress route", func() {
						By("By removing routes and reconciling ingress route using app bundle")
						// DELETE ROUTE
						ab.Spec.Routes = nil

						// UPDATE APPBUNDLE
						er := rec.Update(ctx, ab)
						Expect(er).NotTo(HaveOccurred())
						ApplyTypeMetaToAppBundleForTesting(ab)

						// GET ingress route
						ingressBefore := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err := rec.Get(ctx, client.ObjectKeyFromObject(ingressBefore), ingressBefore)
						Expect(err).NotTo(HaveOccurred())

						// RECONCILE ingress
						err = rec.ReconcileIngress(ctx, fake_req, ab)
						Expect(err).NotTo(HaveOccurred())

						// GET ingress
						ingressAfter := &netv1.Ingress{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err = rec.Get(ctx, client.ObjectKeyFromObject(ingressAfter), ingressAfter)
						Expect(errors.IsNotFound(err)).To(BeTrue())
					})
				})
			})
		})
	})
})
