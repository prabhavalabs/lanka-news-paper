FROM golang:1.25.13-alpine AS build

WORKDIR /src
COPY services/api/go.mod services/api/go.sum ./
RUN go mod download
COPY services/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/content-backfill ./cmd/content-backfill \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22

ARG CODEX_CLI_VERSION=0.147.0

RUN apk add --no-cache ca-certificates nodejs npm su-exec tzdata \
    && npm install --global "@openai/codex@${CODEX_CLI_VERSION}" \
    && npm cache clean --force \
    && addgroup -S app \
    && adduser -S -G app -h /app app \
    && install -d -o app -g app /data/media /data/codex
WORKDIR /app
COPY --from=build /out/ ./
COPY services/api/migrations ./migrations
USER app

ENTRYPOINT ["/app/api"]
