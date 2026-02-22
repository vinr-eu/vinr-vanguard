package aws

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrInvalidMode = errors.New("aws/config: MODE must be 'local' or 'server'")
)

func LoadServiceConfig(ctx context.Context, prefix string) (aws.Config, error) {
	mode := os.Getenv("MODE")
	switch mode {
	case "local", "":
		return loadServiceLocal(ctx, prefix)
	case "server":
		return loadServiceRemote(ctx, prefix)
	default:
		return aws.Config{}, errs.WrapMsg(ErrInvalidMode, "got "+mode)
	}
}

func loadServiceLocal(ctx context.Context, prefix string) (aws.Config, error) {
	endpoint := getServiceEnv(prefix, "AWS_ENDPOINT_URL", "")
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(getServiceEnv(prefix, "AWS_REGION", "us-east-1")),
		config.WithCredentialsProvider(
			aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     getServiceEnv(prefix, "AWS_ACCESS_KEY_ID", "test"),
					SecretAccessKey: getServiceEnv(prefix, "AWS_SECRET_ACCESS_KEY", "test"),
				}, nil
			}),
		),
	}
	if endpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(endpoint))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}

func loadServiceRemote(ctx context.Context, prefix string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(getServiceEnv(prefix, "AWS_REGION", "")),
	)
}

func getServiceEnv(prefix, key, fallback string) string {
	if v, ok := os.LookupEnv(prefix + "_" + key); ok {
		return v
	}
	return getEnv(key, fallback)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
