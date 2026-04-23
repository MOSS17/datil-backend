package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestR2Roundtrip is gated on R2_TEST_BUCKET to avoid running in CI by default.
// Required env: R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY,
// R2_TEST_BUCKET, R2_TEST_PUBLIC_BASE_URL.
func TestR2Roundtrip(t *testing.T) {
	bucket := os.Getenv("R2_TEST_BUCKET")
	if bucket == "" {
		t.Skip("R2_TEST_BUCKET not set; skipping live R2 round-trip")
	}

	uploader, err := NewR2Uploader(R2Config{
		AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		Bucket:          bucket,
		PublicBaseURL:   os.Getenv("R2_TEST_PUBLIC_BASE_URL"),
	})
	if err != nil {
		t.Fatalf("NewR2Uploader: %v", err)
	}

	var seed [4]byte
	_, _ = rand.Read(seed[:])
	key := "tests/" + hex.EncodeToString(seed[:]) + ".bin"
	payload := bytes.Repeat([]byte("R2-OK"), 200)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url, err := uploader.Upload(ctx, key, "application/octet-stream", int64(len(payload)), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if url == "" {
		t.Fatalf("empty url returned")
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, payload) {
		t.Fatalf("body mismatch: got %d bytes want %d", len(got), len(payload))
	}
}
