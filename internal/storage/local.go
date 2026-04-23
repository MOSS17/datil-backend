package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalDiskUploader writes to a local directory and returns URLs served by a
// static handler the caller mounts at publicBaseURL. For development only —
// Railway containers have ephemeral disks, so do not use in production.
type LocalDiskUploader struct {
	Root          string
	PublicBaseURL string
}

func NewLocalDiskUploader(root, publicBaseURL string) (*LocalDiskUploader, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating upload root: %w", err)
	}
	return &LocalDiskUploader{Root: root, PublicBaseURL: strings.TrimRight(publicBaseURL, "/")}, nil
}

func (u *LocalDiskUploader) Upload(_ context.Context, key, _ string, _ int64, r io.Reader) (string, error) {
	clean := filepath.Clean("/" + key)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid key: %q", key)
	}
	dst := filepath.Join(u.Root, filepath.FromSlash(clean))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("creating dir: %w", err)
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return u.PublicBaseURL + clean, nil
}
