package controller

// Test framework setup
import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle", func() {
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
		ApplyTypeMetaToAppBundleForTesting(ab)
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
		er = rec.Update(ctx, ab)
		Expect(er).NotTo(HaveOccurred())

		// RECONCILE
		_, err := rec.Reconcile(ctx, fake_req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should update deployment with new replica count", func() {
		By("Changing the app bundle to have 2 replicas instead of 1")
		// GET the resource
		deploymentBefore := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err := rec.Get(ctx, client.ObjectKeyFromObject(deploymentBefore), deploymentBefore)
		Expect(err).NotTo(HaveOccurred())

		// CHECK the resource
		Expect(deploymentBefore.Name).To(Equal(ab.Name))
		Expect(deploymentBefore.Namespace).To(Equal(ab.Namespace))
		Expect(deploymentBefore.Spec.Replicas).To(Equal(int32(1)))

		// CHANGE AppBundle by adding one more replica
		newReplicas := int32(2)
		ab.Spec.Replicas = &newReplicas
		ApplyTypeMetaToAppBundleForTesting(ab)
		err = rec.Update(ctx, ab)
		Expect(err).NotTo(HaveOccurred())

		// WAIT
		time.Sleep(15 * time.Second) // 10 sec is enough adding 5 for good measure

		// GET the resource
		deploymentAfter := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err = rec.Get(ctx, client.ObjectKeyFromObject(deploymentAfter), deploymentAfter)
		Expect(err).NotTo(HaveOccurred())

		// CHECK the resource
		Expect(deploymentAfter.Name).To(Equal(ab.Name))
		Expect(deploymentAfter.Namespace).To(Equal(ab.Namespace))
		Expect(deploymentAfter.Spec.Replicas).To(Equal(int32(2)))
	})
})
