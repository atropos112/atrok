package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle for just deplyoment", func() {
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

		// RECONCILE
		err := rec.ReconcileDeployment(ctx, ab)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should make a deployment with the correct name", func() {
		By("Reconciling deployment using app bundle")
		// GET the resource
		deployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err := rec.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
		Expect(err).NotTo(HaveOccurred())

		// CHECK the resource
		Expect(deployment.Name).To(Equal(ab.Name))
		Expect(deployment.Namespace).To(Equal(ab.Namespace))
		Expect(deployment.Spec.Selector.MatchLabels[AppBundleSelector]).To(Equal(ab.Name))

		// CHECK the resource
		containers := deployment.Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Name).To(Equal(ab.Name))
		Expect(containers[0].Image).To(Equal(*ab.Spec.Image.Repository + ":" + *ab.Spec.Image.Tag))
		Expect(containers[0].Ports).To(HaveLen(0))
		Expect(containers[0].Env).To(HaveLen(0))
		Expect(containers[0].VolumeMounts).To(HaveLen(0))
	})

	It("Should update the deployment when the app bundle is updated with new image tag", func() {
		By("Changing image tag and reconciling deployment using app bundle")
		// GET the resource
		beforeDeployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err := rec.Get(ctx, client.ObjectKeyFromObject(beforeDeployment), beforeDeployment)
		Expect(err).NotTo(HaveOccurred())

		// CHANGE AppBundle
		newTag := "mainline-alpine"
		ab.Spec.Image.Tag = &newTag
		err = rec.Update(ctx, ab)
		Expect(err).NotTo(HaveOccurred())

		// RECONCILE
		err = rec.ReconcileDeployment(ctx, ab)
		Expect(err).NotTo(HaveOccurred())

		// GET the resource
		afterDeployment := &appsv1.Deployment{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
		err = rec.Get(ctx, client.ObjectKeyFromObject(afterDeployment), afterDeployment)
		Expect(err).NotTo(HaveOccurred())

		// CHECK the resource
		Expect(afterDeployment.Name).To(Equal(ab.Name))
		Expect(afterDeployment.Namespace).To(Equal(ab.Namespace))
		Expect(afterDeployment.Spec.Selector.MatchLabels[AppBundleSelector]).To(Equal(ab.Name))

		beforeContainers := beforeDeployment.Spec.Template.Spec.Containers
		afterContainers := afterDeployment.Spec.Template.Spec.Containers

		Expect(beforeContainers[0].Image).NotTo(Equal(afterContainers[0].Image))
		Expect(afterContainers[0].Image).To(Equal(*ab.Spec.Image.Repository + ":" + *ab.Spec.Image.Tag))
	})

	Describe("Changing the app bundle to a more complex one with volumes and envs and ports", func() {
		Context("And reconciling deployment", func() {
			BeforeEach(func() {
				ab.Spec.Envs = make(map[string]string)
				ab.Spec.Envs["test"] = "test"
				ab.Spec.Envs["test2"] = "test2"

				ab.Spec.Resources = &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu":    resource.MustParse("1500m"),
						"memory": resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("750Mi"),
					},
				}
			})
		})
	})
})
