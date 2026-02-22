package aws

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrSMGetSecret = errors.New("aws/secretsmanager: failed to get secret")
)

type SecretsManagerClient struct {
	client *secretsmanager.Client
}

func NewSecretsManagerClient(cfg aws.Config) *SecretsManagerClient {
	return &SecretsManagerClient{
		client: secretsmanager.NewFromConfig(cfg),
	}
}

func (s *SecretsManagerClient) GetSecret(ctx context.Context, name string) (string, error) {
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", errs.WrapMsgErr(ErrSMGetSecret, name, err)
	}
	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if out.SecretBinary != nil {
		return base64.StdEncoding.EncodeToString(out.SecretBinary), nil
	}
	return "", nil
}
