package spaces

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "s3mc/internal/config"
)

func NewClient(ctx context.Context, cfg appconfig.Config) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.SpacesRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.SpacesKey, cfg.SpacesSecret, "")),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = &cfg.Endpoint
		o.UsePathStyle = true
	}), nil
}
