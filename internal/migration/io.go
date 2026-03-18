package migration

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/chai2010/webp"
)

const (
	defaultRetryCount = 4
)

var pngMagicHeader = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func isPNGObject(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	if isPNGKey(key) {
		return true, nil
	}

	contentType, err := objectContentType(ctx, client, bucket, key)
	if err != nil {
		return false, err
	}
	if isPNGContentType(contentType) {
		return true, nil
	}

	return hasPNGMagicHeader(ctx, client, bucket, key)
}

func objectContentType(ctx context.Context, client *s3.Client, bucket, key string) (string, error) {
	contentType := ""
	err := retry(ctx, 3, 300*time.Millisecond, func() error {
		out, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return err
		}

		contentType = strings.TrimSpace(aws.ToString(out.ContentType))
		return nil
	})
	if err != nil {
		return "", err
	}

	return contentType, nil
}

func isPNGContentType(contentType string) bool {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}

	return strings.EqualFold(mediaType, "image/png")
}

func hasPNGMagicHeader(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	header := []byte(nil)
	err := retry(ctx, 3, 300*time.Millisecond, func() error {
		out, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Range:  aws.String("bytes=0-7"),
		})
		if err != nil {
			if isRangeNotSatisfiable(err) {
				header = nil
				return nil
			}
			return err
		}
		defer out.Body.Close()

		data, err := io.ReadAll(io.LimitReader(out.Body, int64(len(pngMagicHeader))))
		if err != nil {
			return err
		}

		header = data
		return nil
	})
	if err != nil {
		return false, err
	}

	return bytes.Equal(header, pngMagicHeader), nil
}

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

func downloadAndConvertToWebP(ctx context.Context, client *s3.Client, bucket, key string, quality int, lossless bool, exact bool) ([]byte, error) {
	pngData, err := downloadObject(ctx, client, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("download object %q: %w", key, err)
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode png %q: %w", key, err)
	}

	opts := &webp.Options{Quality: float32(quality)}
	if lossless {
		opts.Lossless = true
		opts.Exact = exact
	}

	var encoded bytes.Buffer
	if err := webp.Encode(&encoded, img, opts); err != nil {
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

func uploadWebP(ctx context.Context, client *s3.Client, bucket, key string, data []byte, cacheControl string, publicRead bool) error {
	err := retry(ctx, defaultRetryCount, 500*time.Millisecond, func() error {
		input := &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String("image/webp"),
		}

		if cacheControl != "" {
		if publicRead {
			input.ACL = types.ObjectCannedACLPublicRead
		}

		_, err := client.PutObject(ctx, input)
		return err
	})
	if err != nil {
		return fmt.Errorf("upload object %q: %w", key, err)
	}

	return nil
}
