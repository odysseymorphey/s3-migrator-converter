package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

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
	WebPLossless bool
	WebPExact    bool
	Concurrency  int
	SkipExisting bool
	DryRun       bool
	CacheControl string
	PublicRead   bool
}

type envConfig struct {
	SpacesKey    string `env:"SPACES_KEY"`
	SpacesSecret string `env:"SPACES_SECRET"`
	SpacesRegion string `env:"SPACES_REGION"`
	Endpoint     string `env:"SPACES_ENDPOINT"`

	SourceBucket string `env:"SOURCE_BUCKET"`
	SourcePrefix string `env:"SOURCE_PREFIX"`
	DestBucket   string `env:"DEST_BUCKET"`
	DestPrefix   string `env:"DEST_PREFIX"`

	WebPQuality  int    `env:"WEBP_QUALITY" envDefault:"80"`
	WebPLossless bool   `env:"WEBP_LOSSLESS" envDefault:"false"`
	WebPExact    bool   `env:"WEBP_EXACT" envDefault:"true"`
	Concurrency  int    `env:"CONCURRENCY"`
	SkipExisting bool   `env:"SKIP_EXISTING" envDefault:"true"`
	DryRun       bool   `env:"DRY_RUN" envDefault:"false"`
	CacheControl string `env:"CACHE_CONTROL" envDefault:"public, max-age=3600, s-max-age=86400"`
	PublicRead   bool   `env:"PUBLIC_READ" envDefault:"true"`
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}
	if err := applyAliases(); err != nil {
		return Config{}, err
	}

	var parsed envConfig
	if err := env.Parse(&parsed); err != nil {
		return Config{}, fmt.Errorf("parse env config: %w", err)
	}

	cfg := Config{
		SpacesKey:    strings.TrimSpace(parsed.SpacesKey),
		SpacesSecret: strings.TrimSpace(parsed.SpacesSecret),
		SpacesRegion: strings.TrimSpace(parsed.SpacesRegion),
		Endpoint:     normalizeEndpoint(parsed.Endpoint),

		SourceBucket: strings.TrimSpace(parsed.SourceBucket),
		SourcePrefix: normalizePrefix(parsed.SourcePrefix),
		DestBucket:   strings.TrimSpace(parsed.DestBucket),
		DestPrefix:   normalizePrefix(parsed.DestPrefix),

		WebPQuality:  parsed.WebPQuality,
		WebPLossless: parsed.WebPLossless,
		WebPExact:    parsed.WebPExact,
		Concurrency:  parsed.Concurrency,
		SkipExisting: parsed.SkipExisting,
		DryRun:       parsed.DryRun,
		CacheControl: strings.TrimSpace(parsed.CacheControl),
		PublicRead:   parsed.PublicRead,
	}

	if cfg.DestBucket == "" {
		cfg.DestBucket = cfg.SourceBucket
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = normalizeEndpoint(fmt.Sprintf("https://%s.digitaloceanspaces.com", cfg.SpacesRegion))
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = max(runtime.NumCPU(), 2)
	}
	if cfg.CacheControl == "" {
		cfg.CacheControl = "public, max-age=3600, s-max-age=86400"
	}

	if cfg.WebPQuality < 1 || cfg.WebPQuality > 100 {
		return Config{}, fmt.Errorf("WEBP_QUALITY must be an integer from 1 to 100")
	}
	if cfg.Concurrency < 1 {
		return Config{}, fmt.Errorf("CONCURRENCY must be an integer greater than 0")
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

func applyAliases() error {
	if err := setFromAlias("SPACES_KEY", "AWS_ACCESS_KEY_ID"); err != nil {
		return err
	}
	if err := setFromAlias("SPACES_SECRET", "AWS_SECRET_ACCESS_KEY"); err != nil {
		return err
	}

	return nil
}

func setFromAlias(targetKey, aliasKey string) error {
	if strings.TrimSpace(os.Getenv(targetKey)) != "" {
		return nil
	}

	aliasValue := strings.TrimSpace(os.Getenv(aliasKey))
	if aliasValue == "" {
		return nil
	}

	if err := os.Setenv(targetKey, aliasValue); err != nil {
		return fmt.Errorf("set %s from %s: %w", targetKey, aliasKey, err)
	}

	return nil
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

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
