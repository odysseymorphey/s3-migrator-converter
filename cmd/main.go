package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"s3mc/internal/config"
	"s3mc/internal/migration"
	"s3mc/internal/spaces"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Printf("migration error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log.Printf(
		"starting one-shot migration: source=%s prefix=%q destination=%s prefix=%q quality=%d lossless=%t exact=%t concurrency=%d skip_existing=%t dry_run=%t public_read=%t cache_control=%q",
		cfg.SourceBucket,
		cfg.SourcePrefix,
		cfg.DestBucket,
		cfg.DestPrefix,
		cfg.WebPQuality,
		cfg.WebPLossless,
		cfg.WebPExact,
		cfg.Concurrency,
		cfg.SkipExisting,
		cfg.DryRun,
		cfg.PublicRead,
		cfg.CacheControl,
	)

	client, err := spaces.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create spaces client: %w", err)
	}

	stats, listErr := migration.Run(ctx, client, cfg)
	log.Printf(
		"migration summary: discovered=%d planned=%d converted=%d skipped_existing=%d skipped_non_png=%d skipped_total=%d failed=%d",
		stats.Discovered,
		stats.Planned,
		stats.Converted,
		stats.SkippedExisting,
		stats.SkippedNonPNG,
		stats.SkippedTotal(),
		stats.Failed,
	)

	if listErr != nil {
		return fmt.Errorf("list source objects: %w", listErr)
	}
	if stats.Failed > 0 {
		return fmt.Errorf("migration finished with %d failed objects", stats.Failed)
	}

	return nil
}
