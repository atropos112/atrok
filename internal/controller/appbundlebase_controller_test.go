package controller

// Test framework setup
import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Correctly populated AppBundle for just deplyoment", func() {

	It("Should select first non-default element", func() {
		By("Using the ReturnFirstNonDefault function on two elements")
		strEle1 := ""
		strEle2 := "second"

		intEle1 := 0
		intEle2 := 1

		Expect(ReturnFirstNonDefault(strEle1, strEle2)).To(Equal(strEle2))
		Expect(ReturnFirstNonDefault(intEle1, intEle2)).To(Equal(intEle2))

		strEle1 = "first"
		strEle2 = "second"

		intEle1 = 1
		intEle2 = 2

		Expect(ReturnFirstNonDefault(strEle1, strEle2)).To(Equal(strEle1))
		Expect(ReturnFirstNonDefault(intEle1, intEle2)).To(Equal(intEle1))

		strEle1 = ""
		strEle2 = ""

		intEle1 = 0
		intEle2 = 0

		Expect(ReturnFirstNonDefault(strEle1, strEle2)).To(Equal(strEle2))
		Expect(ReturnFirstNonDefault(intEle1, intEle2)).To(Equal(intEle2))

		listEle1 := []string{"first"}
		listEle2 := []string{"second"}

		listEle3 := []int{1}
		listEle4 := []int{2}

		listEle5 := []int{2}
		listEle6 := []int{}

		Expect(ReturnFirstNonDefault(listEle1, listEle2)).To(Equal(listEle1))
		Expect(ReturnFirstNonDefault(listEle3, listEle4)).To(Equal(listEle3))
		Expect(ReturnFirstNonDefault(listEle5, listEle6)).To(Equal(listEle5))
	})
})
