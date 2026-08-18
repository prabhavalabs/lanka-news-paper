#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: deploy.sh <image-tag>" >&2
  exit 2
fi

cd "$(dirname "$0")"
export IMAGE_TAG="$1"

docker compose pull
docker compose up -d postgres
docker compose --profile tools run --rm migrate
docker compose up -d --remove-orphans --wait

printf '%s\n' "$IMAGE_TAG" > .deployed-image
