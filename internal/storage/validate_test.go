package storage

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var imageTypes = []string{"image/png", "image/jpeg", "image/webp", "application/pdf"}

// Magic-byte prefixes — http.DetectContentType only needs enough to classify.
var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pdfMagic  = []byte("%PDF-1.4\n")
	webpMagic = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 32)...)
	htmlBody  = []byte("<!DOCTYPE html><html><body>hi</body></html>")
)

func TestDetectAndValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		body        []byte
		allow       []string
		wantCT      string
		wantErr     error
		wantContain string
	}{
		{name: "png accepted", body: append(pngMagic, bytes.Repeat([]byte{1}, 1000)...), allow: imageTypes, wantCT: "image/png"},
		{name: "jpeg accepted", body: append(jpegMagic, bytes.Repeat([]byte{1}, 1000)...), allow: imageTypes, wantCT: "image/jpeg"},
		{name: "pdf accepted", body: append(pdfMagic, bytes.Repeat([]byte{1}, 1000)...), allow: imageTypes, wantCT: "application/pdf"},
		{name: "webp accepted", body: webpMagic, allow: imageTypes, wantCT: "image/webp"},
		{name: "html rejected", body: htmlBody, allow: imageTypes, wantErr: ErrInvalidContentType},
		{name: "text rejected", body: []byte("just some plain text content for sniffing"), allow: []string{"image/png"}, wantErr: ErrInvalidContentType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, body, err := DetectAndValidate(bytes.NewReader(tc.body), tc.allow, 0)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ct != tc.wantCT {
				t.Fatalf("content type: want %q, got %q", tc.wantCT, ct)
			}
			// Body must still yield the original payload in full.
			got, err := io.ReadAll(body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			if !bytes.Equal(got, tc.body) {
				t.Fatalf("body roundtrip mismatch: got %d bytes, want %d", len(got), len(tc.body))
			}
		})
	}
}

func TestEnforceSize(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	n, err := EnforceSize(&buf, strings.NewReader("hello"), 10)
	if err != nil {
		t.Fatalf("under limit: %v", err)
	}
	if n != 5 {
		t.Fatalf("want 5 bytes, got %d", n)
	}

	buf.Reset()
	_, err = EnforceSize(&buf, strings.NewReader("123456789012345"), 10)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}
