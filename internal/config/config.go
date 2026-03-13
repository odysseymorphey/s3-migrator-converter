package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const defaultWebPQuality = 80

type Config struct {
	SpacesKey    string
	SpacesSecret string
	SpacesRegion string
	Endpoint     string

	SourceBucket string
	SourcePrefix string
	DestBucket   string
	DestPrefix   string

	WebPQuality  int
	Concurrency  int
	SkipExisting bool
	DryRun       bool
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		SpacesKey:    firstNonEmpty(os.Getenv("SPACES_KEY"), os.Getenv("AWS_ACCESS_KEY_ID")),
		SpacesSecret: firstNonEmpty(os.Getenv("SPACES_SECRET"), os.Getenv("AWS_SECRET_ACCESS_KEY")),
		SpacesRegion: strings.TrimSpace(os.Getenv("SPACES_REGION")),
		SourceBucket: strings.TrimSpace(os.Getenv("SOURCE_BUCKET")),
		SourcePrefix: normalizePrefix(os.Getenv("SOURCE_PREFIX")),
		DestBucket:   strings.TrimSpace(os.Getenv("DEST_BUCKET")),
		DestPrefix:   normalizePrefix(os.Getenv("DEST_PREFIX")),
		WebPQuality:  defaultWebPQuality,
		Concurrency:  max(runtime.NumCPU(), 2),
		SkipExisting: true,
		DryRun:       false,
	}
	if cfg.DestBucket == "" {
		cfg.DestBucket = cfg.SourceBucket
	}

	if raw := strings.TrimSpace(os.Getenv("SPACES_ENDPOINT")); raw != "" {
		cfg.Endpoint = normalizeEndpoint(raw)
	} else {
		cfg.Endpoint = normalizeEndpoint(fmt.Sprintf("https://%s.digitaloceanspaces.com", cfg.SpacesRegion))
	}

	if raw := strings.TrimSpace(os.Getenv("WEBP_QUALITY")); raw != "" {
		q, err := strconv.Atoi(raw)
		if err != nil || q < 1 || q > 100 {
			return Config{}, fmt.Errorf("WEBP_QUALITY must be an integer from 1 to 100")
		}
		cfg.WebPQuality = q
	}

	if raw := strings.TrimSpace(os.Getenv("CONCURRENCY")); raw != "" {
		workers, err := strconv.Atoi(raw)
		if err != nil || workers < 1 {
			return Config{}, fmt.Errorf("CONCURRENCY must be an integer greater than 0")
		}
		cfg.Concurrency = workers
	}

	if raw := strings.TrimSpace(os.Getenv("SKIP_EXISTING")); raw != "" {
		skip, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SKIP_EXISTING must be a boolean value")
		}
		cfg.SkipExisting = skip
	}

	if raw := strings.TrimSpace(os.Getenv("DRY_RUN")); raw != "" {
		dryRun, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("DRY_RUN must be a boolean value")
		}
		cfg.DryRun = dryRun
	}

	var missing []string
	if cfg.SpacesKey == "" {
		missing = append(missing, "SPACES_KEY (or AWS_ACCESS_KEY_ID)")
	}
	if cfg.SpacesSecret == "" {
		missing = append(missing, "SPACES_SECRET (or AWS_SECRET_ACCESS_KEY)")
	}
	if cfg.SpacesRegion == "" {
		missing = append(missing, "SPACES_REGION")
	}
	if cfg.SourceBucket == "" {
		missing = append(missing, "SOURCE_BUCKET")
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func loadDotEnv() error {
	envFile := strings.TrimSpace(os.Getenv("ENV_FILE"))
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("load env file %q: %w", envFile, err)
		}
		return nil
	}

	if _, err := os.Stat(".env"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("check .env file: %w", err)
	}

	if err := godotenv.Load(".env"); err != nil {
		return fmt.Errorf("load .env file: %w", err)
	}

	return nil
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}

	return prefix + "/"
}

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return endpoint
	}

	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {
		return endpoint
	}

	return "https://" + endpoint
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
