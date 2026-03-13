# s3-migrator-converter

One-shot CLI tool that reads PNG files from a source DigitalOcean Spaces bucket,
converts them to `.webp`, and uploads them to a destination bucket.

The tool supports keys without file extension and detects PNG by object metadata/header.

## Project layout

- `cmd/main.go` - application entrypoint
- `internal/config` - environment and `.env` config loading
- `internal/spaces` - S3/Spaces client setup
- `internal/migration` - listing, conversion, upload, retries, worker pool

## PNG detection

For each source object, the app uses an auto strategy:

1. If key extension is `.png`, treat it as PNG.
2. Otherwise check source object `Content-Type` via `HeadObject`.
3. If `Content-Type` is not `image/png`, read first 8 bytes (`Range: bytes=0-7`) and
   verify PNG magic header.

Only confirmed PNG objects are converted.

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
