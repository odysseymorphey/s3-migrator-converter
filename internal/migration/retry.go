package migration

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func retry(ctx context.Context, attempts int, baseDelay time.Duration, fn func() error) error {
	var err error

	for i := 1; i <= attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i == attempts || !isRetryable(err) {
			return err
		}

		delay := time.Duration(i*i) * baseDelay
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return err
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		statusCode := respErr.HTTPStatusCode()
		if statusCode == 429 || statusCode >= 500 {
			return true
		}
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "RequestTimeout", "RequestTimeoutException", "Throttling", "ThrottlingException", "SlowDown", "InternalError", "ServiceUnavailable", "RequestLimitExceeded", "500", "503":
			return true
		}
	}

	return false
}

func isNotFoundError(err error) bool {
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() == 404
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}

	return false
}

func isRangeNotSatisfiable(err error) bool {
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() == 416
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "InvalidRange", "RequestedRangeNotSatisfiable", "416":
			return true
		}
	}

	return false
}
