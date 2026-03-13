package migration

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/chai2010/webp"
)

const (
	defaultRetryCount = 4
)

func objectExists(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	exists := false
	err := retry(ctx, 3, 300*time.Millisecond, func() error {
		_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			if isNotFoundError(err) {
				exists = false
				return nil
			}
			return err
		}

		exists = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

func downloadAndConvertToWebP(ctx context.Context, client *s3.Client, bucket, key string, quality int) ([]byte, error) {
	pngData, err := downloadObject(ctx, client, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("download object %q: %w", key, err)
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode png %q: %w", key, err)
	}

	var encoded bytes.Buffer
	if err := webp.Encode(&encoded, img, &webp.Options{Quality: float32(quality)}); err != nil {
		return nil, fmt.Errorf("encode webp %q: %w", key, err)
	}

	return encoded.Bytes(), nil
}

func downloadObject(ctx context.Context, client *s3.Client, bucket, key string) ([]byte, error) {
	var payload []byte
	err := retry(ctx, defaultRetryCount, 500*time.Millisecond, func() error {
		out, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return err
		}
		defer out.Body.Close()

		data, err := io.ReadAll(out.Body)
		if err != nil {
			return err
		}

		payload = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func uploadWebP(ctx context.Context, client *s3.Client, bucket, key string, data []byte) error {
	err := retry(ctx, defaultRetryCount, 500*time.Millisecond, func() error {
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String("image/webp"),
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("upload object %q: %w", key, err)
	}

	return nil
}
