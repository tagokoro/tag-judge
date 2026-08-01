package awsinfra

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func loadConfig(ctx context.Context, region, endpointURL string) (aws.Config, error) {
	options := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if endpointURL != "" {
		options = append(options, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		))
	}
	return config.LoadDefaultConfig(ctx, options...)
}
