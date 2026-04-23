package storage

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
)

// DetectAndValidate reads up to the first 512 bytes from r to sniff the
// content type via http.DetectContentType, rejects anything outside allowed,
// and returns a new Reader that yields the full original stream (prefix + rest).
//
// If maxBytes > 0 the returned reader is capped at maxBytes+1 so callers can
// reliably detect oversize payloads by attempting one extra byte read.
func DetectAndValidate(r io.Reader, allowed []string, maxBytes int64) (contentType string, body io.Reader, err error) {
	const sniffLen = 512
	prefix := make([]byte, sniffLen)
	n, readErr := io.ReadFull(r, prefix)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return "", nil, fmt.Errorf("reading upload: %w", readErr)
	}
	prefix = prefix[:n]

	contentType = http.DetectContentType(prefix)
	// DetectContentType returns e.g. "image/jpeg; charset=..."; strip params.
	if i := bytes.IndexByte([]byte(contentType), ';'); i >= 0 {
		contentType = contentType[:i]
	}

	if !slices.Contains(allowed, contentType) {
		return contentType, nil, fmt.Errorf("%w: %s", ErrInvalidContentType, contentType)
	}

	combined := io.MultiReader(bytes.NewReader(prefix), r)
	if maxBytes > 0 {
		combined = io.LimitReader(combined, maxBytes+1)
	}
	return contentType, combined, nil
}

// EnforceSize copies src to dst and returns the bytes written, or ErrTooLarge
// if more than maxBytes were consumed.
func EnforceSize(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, maxBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxBytes {
		return n, ErrTooLarge
	}
	return n, nil
}
