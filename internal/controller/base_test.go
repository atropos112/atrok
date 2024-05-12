package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated very basic AppBundle", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleBaseReconciler
	var abRec *AppBundleReconciler
	var ctx context.Context

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		rec = &AppBundleBaseReconciler{Client: k8sClient, Scheme: scheme.Scheme}
		abRec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
	})

	It("Should make no difference when resolving without a base against base", func() {
		By("Applying base resolver to app bundle")

		beforeAb := ab.DeepCopy()
		abb := &atroxyzv1alpha1.AppBundleBase{}
		err := ResolveAppBundleBase(ctx, abRec, ab, abb)
		Expect(err).NotTo(HaveOccurred())

		Expect(ab).To(Equal(beforeAb))
	})

	It("Should append all routes from base to app bundle", func() {
		By("Applying base resolver to app bundle")
		route1_name := "test"
		route1_port := 80
		route1 := atroxyzv1alpha1.AppBundleRoute{Port: &route1_port}

		route2_name := "test2"
		route2_port := 8080
		route2 := atroxyzv1alpha1.AppBundleRoute{Port: &route2_port}

		abbSpec := atroxyzv1alpha1.AppBundleBaseSpec{
			Routes: map[string]atroxyzv1alpha1.AppBundleRoute{
				route1_name: route1,
				route2_name: route2,
			},
		}
		abb := &atroxyzv1alpha1.AppBundleBase{
			ObjectMeta: metav1.ObjectMeta{
				Name: GetRandomName(),
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "atro.xyz/v1alpha1",
				Kind:       "AppBundleBase",
			},
			Spec: abbSpec,
		}
		err := rec.Create(ctx, abb)
		Expect(err).NotTo(HaveOccurred())

		ab.Spec.Base = &abb.Name

		err = ResolveAppBundleBase(ctx, abRec, ab, abb)
		Expect(err).NotTo(HaveOccurred())

		Expect(ab.Spec.Routes).To(HaveLen(2))

	})
})

var _ = Describe("Correctly populated heavily populated AppBundle", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleBaseReconciler
	var abRec *AppBundleReconciler
	var ctx context.Context

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		path1 := "/test"
		path2 := "/test2"

		if ab.Spec.Volumes == nil {
			ab.Spec.Volumes = make(map[string]atroxyzv1alpha1.AppBundleVolume)
		}

		ab.Spec.Volumes["test"] = atroxyzv1alpha1.AppBundleVolume{Path: &path1}
		ab.Spec.Volumes["test2"] = atroxyzv1alpha1.AppBundleVolume{Path: &path2}

		route1_name := "test"
		route1_port := 80
		route1 := atroxyzv1alpha1.AppBundleRoute{Port: &route1_port}
		route2_name := "test2"
		route2_port := 8080
		route2 := atroxyzv1alpha1.AppBundleRoute{Port: &route2_port}

		ab.Spec.Routes = map[string]atroxyzv1alpha1.AppBundleRoute{
			route1_name: route1,
			route2_name: route2,
		}

		pullPolicy := "Always"
		ab.Spec.Image.PullPolicy = &pullPolicy
		group := "test"
		ab.Spec.Homepage = &atroxyzv1alpha1.AppBundleHomePage{Group: &group}

		rec = &AppBundleBaseReconciler{Client: k8sClient, Scheme: scheme.Scheme}
		abRec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)
	})

	It("Should make no difference when resolving without a base against base", func() {
		By("Applying base resolver to app bundle")

		beforeAb := ab.DeepCopy()
		abb := &atroxyzv1alpha1.AppBundleBase{}
		err := ResolveAppBundleBase(ctx, abRec, ab, abb)
		Expect(err).NotTo(HaveOccurred())

		Expect(ab).To(Equal(beforeAb))
	})

	It("Should append all routes from base to app bundle", func() {
		By("Applying base resolver to app bundle")
		route1_name := "test"
		route1_port := 80
		route1 := atroxyzv1alpha1.AppBundleRoute{Port: &route1_port}
		route2_name := "test2"
		route2_port := 8080
		route2 := atroxyzv1alpha1.AppBundleRoute{Port: &route2_port}
		abbSpec := atroxyzv1alpha1.AppBundleBaseSpec{
			Routes: map[string]atroxyzv1alpha1.AppBundleRoute{
				route1_name: route1,
				route2_name: route2,
			},
		}
		abb := &atroxyzv1alpha1.AppBundleBase{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "atro.xyz/v1alpha1",
				Kind:       "AppBundleBase",
			},
			Spec: abbSpec,
		}
		err := rec.Create(ctx, abb)
		Expect(err).NotTo(HaveOccurred())

		ab.Spec.Base = &abb.Name

		err = ResolveAppBundleBase(ctx, abRec, ab, abb)
		Expect(err).NotTo(HaveOccurred())

		Expect(ab.Spec.Routes).To(HaveLen(2))

	})
})
