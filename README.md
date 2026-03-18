# s3-migrator-converter

One-shot CLI tool that reads PNG files from a source DigitalOcean Spaces bucket,
converts them to `.webp`, and uploads them to a destination bucket.

Uploaded `.webp` objects are stored with configurable ACL/cache headers.
By default this is `ACL=public-read` and
`Cache-Control: public, max-age=3600, s-max-age=86400`.

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

Optional variables:

- `DEST_BUCKET` (defaults to `SOURCE_BUCKET`)
- `SOURCE_PREFIX`
- `DEST_PREFIX`
- `SPACES_ENDPOINT`
- `WEBP_QUALITY` (1..100, default `80`)
- `WEBP_LOSSLESS` (default `false`; when `true`, encodes WebP in lossless mode)
- `CONCURRENCY` (default `max(CPU, 2)`)
- `SKIP_EXISTING` (default `true`)
- `DRY_RUN` (default `false`)
- `CACHE_CONTROL` (default `public, max-age=3600, s-max-age=86400`)
- `PUBLIC_READ` (default `true`)
- `ENV_FILE` (custom path to env file)

## Same bucket, different folder

You can migrate within one bucket by leaving `DEST_BUCKET` empty and using prefixes:

```bash
SOURCE_BUCKET=my-bucket
DEST_BUCKET=
SOURCE_PREFIX=/folder
DEST_PREFIX=/folder-webp
```

## Run

```bash
go run ./cmd
```

Dry run mode (plan only, no uploads):

```bash
DRY_RUN=true go run ./cmd
```

## Run with Docker Compose

`docker compose` builds a multi-stage image (`golang:1.25-alpine` builder + Alpine runtime)
and injects variables from `.env` via `env_file`.

```bash
docker compose run --rm migrator
```

Dry run with Docker Compose:

```bash
docker compose run --rm -e DRY_RUN=true migrator
```
