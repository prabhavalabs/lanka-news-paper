# Packages artifacts that CI compiled on the runner (see .github/workflows/deploy-production.yml).
# Nothing is compiled here, so only the small COPY layers change between releases.
# For a self-contained build from source, use api.Dockerfile and frontend.Dockerfile instead.

FROM alpine:3.22 AS api

ARG CODEX_CLI_VERSION=0.162.1

RUN apk add --no-cache ca-certificates nodejs npm su-exec tzdata chromium font-noto-sinhala \
    && npm install --global "@openai/codex@${CODEX_CLI_VERSION}" \
    && npm cache clean --force \
    && addgroup -S app \
    && adduser -S -G app -h /app app \
    && install -d -o app -g app /data/media /data/codex
ENV SNAP_SOCIAL_CHROME_PATH=/usr/bin/chromium
ENV SNAP_SOCIAL_CHROME_NO_SANDBOX=true
WORKDIR /app
COPY services/api/migrations ./migrations
COPY .ci-out/api/ ./
USER app

ENTRYPOINT ["/app/api"]

FROM nginx:1.29-alpine AS frontend
COPY infra/production/frontend.conf /etc/nginx/conf.d/default.conf

FROM frontend AS web
COPY .ci-out/web/ /usr/share/nginx/html/

FROM frontend AS admin
COPY .ci-out/admin/ /usr/share/nginx/html/
