package awsinfra

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type SourceCodeStorage struct {
	client *s3.Client
	bucket string
}

func NewSourceCodeStorage(ctx context.Context, region, endpointURL, bucket string) (*SourceCodeStorage, error) {
	cfg, err := loadConfig(ctx, region, endpointURL)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		if endpointURL != "" {
			options.BaseEndpoint = aws.String(endpointURL)
			options.UsePathStyle = true
		}
	})

	return &SourceCodeStorage{client: client, bucket: bucket}, nil
}

func (s *SourceCodeStorage) PutSourceCode(ctx context.Context, params repository.PutSourceCodeParams) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(params.Key),
		Body:        strings.NewReader(params.SourceCode),
		ContentType: aws.String("text/plain; charset=utf-8"),
	})
	return err
}
