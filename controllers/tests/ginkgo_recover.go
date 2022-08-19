package tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
)

func RecoveringGinkgoT() GinkgoTInterface {
	return recoveringGinkgoT{GinkgoT(4)} //4 sets the code-location offset to the right value.... i think ;)
}

type recoveringGinkgoT struct {
	GinkgoT GinkgoTInterface
}

func (t recoveringGinkgoT) Setenv(key, value string) {
	t.GinkgoT.Setenv(key, value)
}

func (t recoveringGinkgoT) Cleanup(f func()) {
	t.GinkgoT.Cleanup(f)
}

func (t recoveringGinkgoT) Error(args ...interface{}) {
	defer GinkgoRecover()
	t.GinkgoT.Error(args...)
	fmt.Fprintln(GinkgoWriter, args...)
}

func (t recoveringGinkgoT) Errorf(format string, args ...interface{}) {
	defer GinkgoRecover()
	t.GinkgoT.Errorf(format, args...)
	fmt.Fprintln(GinkgoWriter, args...)
}

func (t recoveringGinkgoT) Fail() {
	defer GinkgoRecover()
	t.GinkgoT.Fail()
}

func (t recoveringGinkgoT) FailNow() {
	defer GinkgoRecover()
	t.GinkgoT.FailNow()
}

func (t recoveringGinkgoT) Failed() bool {
	return t.GinkgoT.Failed()
}

func (t recoveringGinkgoT) Fatal(args ...interface{}) {
	defer GinkgoRecover()
	t.GinkgoT.Fatal(args...)
	fmt.Fprintln(GinkgoWriter, args...)
}

func (t recoveringGinkgoT) Fatalf(format string, args ...interface{}) {
	defer GinkgoRecover()
	t.GinkgoT.Fatalf(format, args...)
}

func (t recoveringGinkgoT) Helper() {
	t.GinkgoT.Helper()
}

func (t recoveringGinkgoT) Log(args ...interface{}) {
	t.GinkgoT.Log(args...)
	fmt.Fprintln(GinkgoWriter, args...)
}

func (t recoveringGinkgoT) Logf(format string, args ...interface{}) {
	t.GinkgoT.Logf(format, args...)
	fmt.Fprintln(GinkgoWriter, args...)
}

func (t recoveringGinkgoT) Name() string {
	return t.GinkgoT.Name()
}

func (t recoveringGinkgoT) Parallel() {
	t.GinkgoT.Parallel()
}

func (t recoveringGinkgoT) Skip(args ...interface{}) {
	t.GinkgoT.Skip(args...)
}

func (t recoveringGinkgoT) SkipNow() {
	t.GinkgoT.SkipNow()
}

func (t recoveringGinkgoT) Skipf(format string, args ...interface{}) {
	t.GinkgoT.Skipf(format, args...)
}

func (t recoveringGinkgoT) Skipped() bool {
	return t.GinkgoT.Skipped()
}

func (t recoveringGinkgoT) TempDir() string {
	return t.GinkgoT.TempDir()
}
