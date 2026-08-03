package judgeinfra

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type S3SourceCodeStorage struct {
	client *s3.Client
	bucket string
}

func NewS3SourceCodeStorage(cfg aws.Config, bucket string, endpointURL string) *S3SourceCodeStorage {
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		if endpointURL != "" {
			options.BaseEndpoint = aws.String(endpointURL)
			options.UsePathStyle = true
		}
	})
	return &S3SourceCodeStorage{client: client, bucket: bucket}
}

func (s *S3SourceCodeStorage) PutSourceCode(ctx context.Context, params repository.PutSourceCodeParams) error {
	if strings.TrimSpace(s.bucket) == "" {
		return fmt.Errorf("judge storage bucket must not be empty")
	}
	if strings.TrimSpace(params.Key) == "" {
		return fmt.Errorf("source code key must not be empty")
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(params.Key),
		Body:        strings.NewReader(params.SourceCode),
		ContentType: aws.String(sourceContentType(params.Language)),
	})
	if err != nil {
		return fmt.Errorf("put source code to s3://%s/%s: %w", s.bucket, params.Key, err)
	}
	return nil
}

func sourceContentType(language domain.ExecutionLanguage) string {
	switch language {
	case domain.ExecutionLanguagePython:
		return "text/x-python; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
