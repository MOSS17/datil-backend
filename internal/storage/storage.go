// Package storage provides a pluggable object-storage interface used for
// business logos and customer payment proofs. The concrete implementation is
// selected by config.StorageProvider at startup.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrTooLarge is returned when an upload exceeds the configured size limit.
var ErrTooLarge = errors.New("upload exceeds maximum size")

// ErrInvalidContentType is returned when detected content type is not allowed.
var ErrInvalidContentType = errors.New("invalid content type")

// Uploader persists a blob under key and returns a publicly-reachable URL.
//
// Implementations must:
//   - Not trust caller-provided content types; callers detect+validate first.
//   - Return a URL the frontend can fetch without additional auth.
//   - Treat key as opaque; keys are expected to include a random component.
type Uploader interface {
	Upload(ctx context.Context, key, contentType string, size int64, r io.Reader) (publicURL string, err error)
}
