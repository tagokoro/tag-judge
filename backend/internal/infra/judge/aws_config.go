package judgeinfra

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func LoadAWSConfig(ctx context.Context, region string, endpointURL string) (aws.Config, error) {
	options := []func(*awssdkconfig.LoadOptions) error{
		awssdkconfig.WithRegion(region),
	}
	if endpointURL != "" {
		options = append(options, awssdkconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		))
	}
	return awssdkconfig.LoadDefaultConfig(ctx, options...)
}
