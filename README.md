# s3-migrator-converter

One-shot CLI tool that reads `.png` files from a source DigitalOcean Spaces bucket,
converts them to `.webp`, and uploads them to a destination bucket.

## Project layout

- `cmd/main.go` - application entrypoint
- `internal/config` - environment and `.env` config loading
- `internal/spaces` - S3/Spaces client setup
- `internal/migration` - listing, conversion, upload, retries, worker pool

## Configuration

The app loads variables from:

1. Process environment
2. `.env` file (if present)

You can also specify a custom env file with `ENV_FILE`.

Copy `.env.example` to `.env` and fill the values:

```bash
cp .env.example .env
```

Required variables:

- `SPACES_KEY`
- `SPACES_SECRET`
- `SPACES_REGION`
- `SOURCE_BUCKET`
- `DEST_BUCKET`

Optional variables:

- `SOURCE_PREFIX`
- `DEST_PREFIX`
- `SPACES_ENDPOINT`
- `WEBP_QUALITY` (1..100, default `80`)
- `CONCURRENCY` (default `max(CPU, 2)`)
- `SKIP_EXISTING` (default `true`)
- `ENV_FILE` (custom path to env file)

## Run

```bash
go run ./cmd
```
