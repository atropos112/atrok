package controller

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	longhornv1beta2 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	. "github.com/onsi/ginkgo/v2" //lint:ignore ST1001 we need to use ginkgo
	. "github.com/onsi/gomega"    //lint:ignore ST1001 we need to use ginkgo
	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	//+kubebuilder:scaffold:imports
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = atroxyzv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clientgoscheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = longhornv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "devel"}}
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func GetBasicAppBundle() *atroxyzv1alpha1.AppBundle {
	rep := "nginx"
	tag := "latest"
	basicImage := &atroxyzv1alpha1.AppBundleImage{
		Repository: &rep,
		Tag:        &tag,
	}
	name := GetRandomName()

	basicAppBundle := &atroxyzv1alpha1.AppBundle{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "devel"},
		Spec: atroxyzv1alpha1.AppBundleSpec{
			Image: basicImage,
		},
	}

	return basicAppBundle
}

func ApplyTypeMetaToAppBundleForTesting(ab *atroxyzv1alpha1.AppBundle) *atroxyzv1alpha1.AppBundle {
	ab.TypeMeta = metav1.TypeMeta{Kind: "AppBundle", APIVersion: "atroxyz.atrok.io/v1alpha1"}
	return ab
}

func RandStringRunes(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func GetRandomName() string {
	return "tst" + RandStringRunes(5)
}
