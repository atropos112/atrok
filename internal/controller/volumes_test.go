package controller

// Test framework setup
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle with PVC, Emptydir and HostPath", func() {
	var ab *atroxyzv1alpha1.AppBundle
	var rec *AppBundleReconciler
	var ctx context.Context
	// NOTE: I am picking names here so that they are in order (alphabetical, expect pvc to be first, then empty dir then host path), the order is ALSO important and tested here.
	pvcNameInAppBundle := "apvc"
	var pvcName string
	emptyDirName := "bemptydir"
	hostPathName := "chostpath"

	BeforeEach(func() {
		// SETUP
		ctx = context.Background()
		ab = GetBasicAppBundle()
		pvcName = ab.Name + "-" + pvcNameInAppBundle
		size := "1Gi"
		hostPath := "/tmp"
		path1 := "/tmp1"
		path2 := "/tmp2"
		path3 := "/tmp3"
		emptyDir := true

		ab.Spec.Volumes = map[string]atroxyzv1alpha1.AppBundleVolume{
			pvcNameInAppBundle: {Size: &size, Path: &path1},
			emptyDirName:       {EmptyDir: &emptyDir, Path: &path2},
			hostPathName:       {HostPath: &hostPath, Path: &path3},
		}

		rec = &AppBundleReconciler{Client: k8sClient, Scheme: scheme.Scheme}

		// CREATE APPBUNDLE
		er := rec.Create(ctx, ab)
		Expect(er).NotTo(HaveOccurred())
		ApplyTypeMetaToAppBundleForTesting(ab)

		// RECONCILE Volume
		err := rec.ReconcileVolumes(ctx, ab)
		Expect(err).NotTo(HaveOccurred())

		// RECONCILE Deployment
		err = rec.ReconcileDeployment(ctx, ab)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should make a PVC", func() {
		By("Reconciling volumes using app bundle")
		// GET the resources
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ab.Namespace,
		}}
		er := k8sClient.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
		Expect(er).NotTo(HaveOccurred())
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
	})

	It("Shouldn't make a PVC for an emptydir volume", func() {
		By("Reconciling volumes using app bundle")
		// GET the resources
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name:      emptyDirName,
			Namespace: ab.Namespace,
		}}
		er := k8sClient.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
		Expect(er).To(HaveOccurred())
		Expect(er.Error()).To(ContainSubstring("not found"))
	})

	It("Shouldn't make a PVC for a hostpath volume", func() {
		By("Reconciling volumes using app bundle")
		// GET the resources
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name:      hostPathName,
			Namespace: ab.Namespace,
		}}
		er := k8sClient.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
		Expect(er).To(HaveOccurred())
		Expect(er.Error()).To(ContainSubstring("not found"))
	})

	It("Should have a deployment with 3 volumes", func() {
		By("Reconciling volumes using app bundle")
		// GET the resources
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      ab.Name,
			Namespace: ab.Namespace,
		}}

		er := k8sClient.Get(ctx, client.ObjectKey{Name: deployment.Name, Namespace: deployment.Namespace}, deployment)
		Expect(er).NotTo(HaveOccurred())

		// VOLUME MOUNTS
		volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
		Expect(volumeMounts).To(HaveLen(3))
		Expect(volumeMounts[0].Name).To(Equal(pvcNameInAppBundle))
		Expect(volumeMounts[1].Name).To(Equal(emptyDirName))
		Expect(volumeMounts[2].Name).To(Equal(hostPathName))

		// VOLUMES
		volumes := deployment.Spec.Template.Spec.Volumes
		Expect(volumes).To(HaveLen(3))

		// PVC
		Expect(volumes[0].Name).To(Equal(pvcNameInAppBundle))
		Expect(volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal(pvcName))

		// EMPTYDIR
		Expect(volumes[1].Name).To(Equal(emptyDirName))
		Expect(volumes[1].VolumeSource.EmptyDir).NotTo(BeNil())

		// HOSTPATH
		Expect(volumes[2].Name).To(Equal(hostPathName))
		Expect(volumes[2].VolumeSource.HostPath).NotTo(BeNil())
	})
})
