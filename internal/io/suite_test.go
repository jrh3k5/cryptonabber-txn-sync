package io_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIO(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal IO Suite")
}
