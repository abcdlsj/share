package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"notionbot/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Uploader struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
	keyPrefix     string
}

func NewS3Uploader(ctx context.Context, cfg config.Config) (*S3Uploader, error) {
	endpoint := cfg.S3Endpoint

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")),
		awsconfig.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
				}
				return aws.Endpoint{}, fmt.Errorf("unknown service endpoint requested: %s", service)
			},
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle
	})

	return &S3Uploader{
		client:        client,
		bucket:        cfg.S3Bucket,
		publicBaseURL: strings.TrimRight(cfg.S3PublicBaseURL, "/"),
		keyPrefix:     strings.Trim(cfg.S3KeyPrefix, "/"),
	}, nil
}

func (u *S3Uploader) UploadPublic(ctx context.Context, chatID int64, r io.Reader, contentType string) (string, error) {
	key := fmt.Sprintf("%s/%d/%s.jpg", u.keyPrefix, chatID, time.Now().UTC().Format("20060102T150405.000000000"))

	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	base, err := url.Parse(u.publicBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid s3_public_base_url: %w", err)
	}
	base.Path = path.Join(base.Path, key)
	return base.String(), nil
}
