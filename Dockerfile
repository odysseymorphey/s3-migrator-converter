FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/s3mc ./cmd

FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/s3mc /usr/local/bin/s3mc

ENTRYPOINT ["s3mc"]
