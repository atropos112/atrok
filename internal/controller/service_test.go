package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle with no routes reconcilling service", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleReconciler
	var ctx context.Context

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		rec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
	})

	It("Should make no service as there are no routes", func() {
		By("Reconciling service using app bundle")
		err := rec.ReconcileService(ctx, ab)
		Expect(err).NotTo(HaveOccurred())

		// GET the resource
		service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err = rec.Get(ctx, client.ObjectKeyFromObject(service), service)
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	Describe("Adding a single route to AppBundle", func() {
		Context("And updating", func() {
			BeforeEach(func() {
				//ADD ROUTE
				route_name := "test"
				port := 80
				route := atroxyzv1alpha1.AppBundleRoute{Port: &port}

				if ab.Spec.Routes == nil {
					ab.Spec.Routes = map[string]atroxyzv1alpha1.AppBundleRoute{}
				}

				ab.Spec.Routes[route_name] = route

				// UPDATE APPBUNDLE
				er := rec.Update(ctx, ab)
				Expect(er).NotTo(HaveOccurred())
				ApplyTypeMetaToAppBundleForTesting(ab)
			})

			It("Should make a simple service", func() {
				By("Reconciling service using app bundle")
				err := rec.ReconcileService(ctx, ab)
				Expect(err).NotTo(HaveOccurred())

				// GET the resource
				service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
				err = rec.Get(ctx, client.ObjectKeyFromObject(service), service)
				Expect(err).NotTo(HaveOccurred())

				// CHECK the resource
				route_name := "test"
				Expect(service.Spec.Ports).To(HaveLen(1))
				Expect(service.Spec.Ports[0].Port).To(Equal(int32(*ab.Spec.Routes[route_name].Port)))
				Expect(service.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(*ab.Spec.Routes[route_name].Port)))
				Expect(service.Spec.Ports[0].Name).To(Equal(route_name))
			})

			Describe("With a single route", func() {
				Context("And created service", func() {
					BeforeEach(func() {
						err := rec.ReconcileService(ctx, ab)
						Expect(err).NotTo(HaveOccurred())
					})

					It("Should not change the service", func() {
						By("Reconciling service again using app bundle")

						// GET service
						serviceBefore := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err := rec.Get(ctx, client.ObjectKeyFromObject(serviceBefore), serviceBefore)
						Expect(err).NotTo(HaveOccurred())

						// RECONCILE service
						err = rec.ReconcileService(ctx, ab)
						Expect(err).NotTo(HaveOccurred())

						// GET service
						serviceAfter := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err = rec.Get(ctx, client.ObjectKeyFromObject(serviceAfter), serviceAfter)
						Expect(err).NotTo(HaveOccurred())

						// CHECK service
						Expect(serviceBefore).To(Equal(serviceAfter))
					})

					It("Should delete the service", func() {
						By("By removing routes and reconciling service using app bundle")
						// DELETE ROUTE
						ab.Spec.Routes = nil

						// UPDATE APPBUNDLE
						er := rec.Update(ctx, ab)
						Expect(er).NotTo(HaveOccurred())
						ApplyTypeMetaToAppBundleForTesting(ab)

						// GET service
						serviceBefore := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err := rec.Get(ctx, client.ObjectKeyFromObject(serviceBefore), serviceBefore)
						Expect(err).NotTo(HaveOccurred())

						// RECONCILE service
						err = rec.ReconcileService(ctx, ab)
						Expect(err).NotTo(HaveOccurred())

						// GET service
						serviceAfter := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
						err = rec.Get(ctx, client.ObjectKeyFromObject(serviceAfter), serviceAfter)
						Expect(errors.IsNotFound(err)).To(BeTrue())
					})
				})
			})
		})
	})
})

var _ = Describe("AppBundle with incorrectly populated route", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleReconciler
	var ctx context.Context

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		rec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE bad route (negative port)
		route_name := "test"
		port := -19
		route := atroxyzv1alpha1.AppBundleRoute{Port: &port}

		if ab.Spec.Routes == nil {
			ab.Spec.Routes = map[string]atroxyzv1alpha1.AppBundleRoute{}
		}

		ab.Spec.Routes[route_name] = route

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
	})

	It("Should not make a service", func() {
		By("Reconciling service using app bundle")
		err := rec.ReconcileService(ctx, ab)
		Expect(err).To(HaveOccurred())

		// GET the resource
		service := &corev1.Service{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err = rec.Get(ctx, client.ObjectKeyFromObject(service), service)
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})
})
