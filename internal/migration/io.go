package migration

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func detectObjectFormat(ctx context.Context, client *s3.Client, bucket, key string, formats []string) (*imageFormat, error) {
	if f := detectFormat(key, "", nil, formats); f != nil {
		return f, nil
	}

	contentType, err := objectContentType(ctx, client, bucket, key)
	if err != nil {
		return nil, err
	}
	if f := detectFormat(key, contentType, nil, formats); f != nil {
		return f, nil
	}

	magicLen := maxMagicLen(formats)
	if magicLen == 0 {
		return nil, nil
	}

	header, err := readMagicHeader(ctx, client, bucket, key, magicLen)
	if err != nil {
		return nil, err
	}

	return detectFormat(key, contentType, header, formats), nil
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

func readMagicHeader(ctx context.Context, client *s3.Client, bucket, key string, length int) ([]byte, error) {
	var header []byte
	err := retry(ctx, 3, 300*time.Millisecond, func() error {
		out, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Range:  aws.String(fmt.Sprintf("bytes=0-%d", length-1)),
		})
		if err != nil {
			if isRangeNotSatisfiable(err) {
				header = nil
				return nil
			}
			return err
		}
		defer out.Body.Close()

		data, err := io.ReadAll(io.LimitReader(out.Body, int64(length)))
		if err != nil {
			return err
		}

		header = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	return header, nil
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

func downloadAndConvert(ctx context.Context, client *s3.Client, bucket, key string, format *imageFormat, quality int, lossless bool, exact bool) ([]byte, error) {
	data, err := downloadObject(ctx, client, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("download object %q: %w", key, err)
	}

	img, err := format.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s %q: %w", format.Name, key, err)
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
			input.CacheControl = aws.String(cacheControl)
		}
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
