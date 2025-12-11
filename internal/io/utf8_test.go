package io_test

import (
    "bytes"
    "io"

    iopkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/io"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("StripUTF8BOM", func() {
    It("removes a leading UTF-8 BOM when present", func() {
        src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
        r := iopkg.StripUTF8BOM(bytes.NewReader(src))
        b, err := io.ReadAll(r)
        Expect(err).ToNot(HaveOccurred())
        Expect(string(b)).To(Equal("hello"))
    })

    It("returns the original content when no BOM is present", func() {
        src := []byte("world")
        r := iopkg.StripUTF8BOM(bytes.NewReader(src))
        b, err := io.ReadAll(r)
        Expect(err).ToNot(HaveOccurred())
        Expect(string(b)).To(Equal("world"))
    })

    It("handles short readers correctly (less than 3 bytes)", func() {
        src := []byte{0xEF, 0xBB} // incomplete BOM
        r := iopkg.StripUTF8BOM(bytes.NewReader(src))
        b, err := io.ReadAll(r)
        Expect(err).ToNot(HaveOccurred())
        Expect(b).To(Equal(src))
    })

    It("handles an empty reader without error", func() {
        r := iopkg.StripUTF8BOM(bytes.NewReader(nil))
        b, err := io.ReadAll(r)
        Expect(err).ToNot(HaveOccurred())
        Expect(len(b)).To(Equal(0))
    })
})
