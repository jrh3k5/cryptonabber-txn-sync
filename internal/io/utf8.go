package io

import (
	"bufio"
	"bytes"
	"io"
)

// stripUTF8BOM returns an io.Reader that will consume a leading UTF-8 BOM
// (0xEF,0xBB,0xBF) if present, otherwise it returns a buffered reader over r.
func StripUTF8BOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	// Peek does not advance the reader
	b, err := br.Peek(3)
	if err == nil && bytes.Equal(b, []byte{0xEF, 0xBB, 0xBF}) {
		// discard the BOM bytes
		_, _ = br.Discard(3)
	}
	return br
}
