FROM golang:1.25.13-alpine

ARG CODEX_CLI_VERSION=0.147.0

RUN apk add --no-cache ca-certificates nodejs npm tzdata \
    && npm install --global "@openai/codex@${CODEX_CLI_VERSION}" \
    && npm cache clean --force

WORKDIR /workspace/services/api

CMD ["go", "run", "./cmd/worker"]
