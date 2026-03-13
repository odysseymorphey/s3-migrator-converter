package migration

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "s3mc/internal/config"
)

type Stats struct {
	Discovered      int64
	Planned         int64
	Converted       int64
	SkippedExisting int64
	SkippedNonPNG   int64
	Failed          int64
}

func (s Stats) SkippedTotal() int64 {
	return s.SkippedExisting + s.SkippedNonPNG
}

type resultStatus string

const (
	statusConverted       resultStatus = "converted"
	statusPlanned         resultStatus = "planned"
	statusSkippedExisting resultStatus = "skipped_existing"
	statusSkippedNonPNG   resultStatus = "skipped_non_png"
	statusFailed          resultStatus = "failed"
)

type fileResult struct {
	sourceKey string
	destKey   string
	status    resultStatus
	err       error
}

func Run(ctx context.Context, client *s3.Client, cfg appconfig.Config) (Stats, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan string, cfg.Concurrency*2)
	results := make(chan fileResult, cfg.Concurrency*2)

	listDone := make(chan struct {
		discovered int64
		err        error
	}, 1)

	var workersWG sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			worker(ctx, client, cfg, jobs, results)
		}()
	}

	go func() {
		discovered, err := enqueueKeys(ctx, client, cfg, jobs)
		if err != nil {
			cancel()
		}
		close(jobs)
		listDone <- struct {
			discovered int64
			err        error
		}{
			discovered: discovered,
			err:        err,
		}
	}()

	go func() {
		workersWG.Wait()
		close(results)
	}()

	stats := Stats{}
	for res := range results {
		switch res.status {
		case statusPlanned:
			stats.Planned++
			log.Printf("planned (dry-run): %s -> %s", res.sourceKey, res.destKey)
		case statusConverted:
			stats.Converted++
			log.Printf("converted: %s -> %s", res.sourceKey, res.destKey)
		case statusSkippedExisting:
			stats.SkippedExisting++
			log.Printf("skipped (already exists): %s", res.destKey)
		case statusSkippedNonPNG:
			stats.SkippedNonPNG++
			log.Printf("skipped (not png): %s", res.sourceKey)
		case statusFailed:
			stats.Failed++
			log.Printf("failed: %s -> %s: %v", res.sourceKey, res.destKey, res.err)
		}
	}

	listOutcome := <-listDone
	stats.Discovered = listOutcome.discovered
	if listOutcome.err != nil {
		return stats, listOutcome.err
	}

	return stats, nil
}

func enqueueKeys(ctx context.Context, client *s3.Client, cfg appconfig.Config, jobs chan<- string) (int64, error) {
	input := &s3.ListObjectsV2Input{Bucket: aws.String(cfg.SourceBucket)}
	if cfg.SourcePrefix != "" {
		input.Prefix = aws.String(cfg.SourcePrefix)
	}

	paginator := s3.NewListObjectsV2Paginator(client, input)
	var discovered int64

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return discovered, err
		}

		for _, obj := range page.Contents {
			key := strings.TrimSpace(aws.ToString(obj.Key))
			if key == "" || strings.HasSuffix(key, "/") {
				continue
			}
			if shouldSkipDestinationKey(cfg, key) {
				continue
			}

			select {
			case jobs <- key:
				discovered++
			case <-ctx.Done():
				return discovered, ctx.Err()
			}
		}
	}

	return discovered, nil
}

func shouldSkipDestinationKey(cfg appconfig.Config, key string) bool {
	if cfg.SourceBucket != cfg.DestBucket {
		return false
	}
	if cfg.DestPrefix == "" || cfg.DestPrefix == cfg.SourcePrefix {
		return false
	}

	return strings.HasPrefix(key, cfg.DestPrefix)
}

func worker(ctx context.Context, client *s3.Client, cfg appconfig.Config, jobs <-chan string, results chan<- fileResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case sourceKey, ok := <-jobs:
			if !ok {
				return
			}

			res := migrateOne(ctx, client, cfg, sourceKey)
			select {
			case results <- res:
			case <-ctx.Done():
				return
			}
		}
	}
}

func migrateOne(ctx context.Context, client *s3.Client, cfg appconfig.Config, sourceKey string) fileResult {
	destKey := destinationKey(sourceKey, cfg.SourcePrefix, cfg.DestPrefix)
	res := fileResult{
		sourceKey: sourceKey,
		destKey:   destKey,
		status:    statusFailed,
	}

	isPNG, err := isPNGObject(ctx, client, cfg.SourceBucket, sourceKey)
	if err != nil {
		res.err = fmt.Errorf("detect png object: %w", err)
		return res
	}
	if !isPNG {
		res.status = statusSkippedNonPNG
		return res
	}

	if cfg.SkipExisting {
		exists, err := objectExists(ctx, client, cfg.DestBucket, destKey)
		if err != nil {
			res.err = fmt.Errorf("check destination object: %w", err)
			return res
		}
		if exists {
			res.status = statusSkippedExisting
			return res
		}
	}

	if cfg.DryRun {
		res.status = statusPlanned
		return res
	}

	webpData, err := downloadAndConvertToWebP(ctx, client, cfg.SourceBucket, sourceKey, cfg.WebPQuality)
	if err != nil {
		res.err = err
		return res
	}

	if err := uploadWebP(ctx, client, cfg.DestBucket, destKey, webpData); err != nil {
		res.err = err
		return res
	}

	res.status = statusConverted
	return res
}
