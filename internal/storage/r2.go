package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Config holds the credentials and bucket parameters for a Cloudflare R2 client.
// AccountID is used to derive the S3-compatible endpoint.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	// PublicBaseURL is the externally-reachable prefix (custom domain or
	// r2.dev URL). The uploader appends "/<key>" to it.
	PublicBaseURL string
}

// R2Uploader uploads to a Cloudflare R2 bucket via the S3-compatible API.
type R2Uploader struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

func NewR2Uploader(cfg R2Config) (*R2Uploader, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" || cfg.PublicBaseURL == "" {
		return nil, fmt.Errorf("r2: AccountID, AccessKeyID, SecretAccessKey, Bucket, PublicBaseURL all required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: loading aws config: %w", err)
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2Uploader{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (u *R2Uploader) Upload(ctx context.Context, key, contentType string, size int64, r io.Reader) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	if _, err := u.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("r2: putting object: %w", err)
	}
	return u.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
}
