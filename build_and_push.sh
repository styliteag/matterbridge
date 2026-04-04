#!/usr/bin/env bash
set -euo pipefail

IMAGE="styliteag/matterbridge2"
PLATFORMS="linux/amd64,linux/arm64"

# Ensure a multi-platform builder exists
if ! docker buildx inspect multiplatform >/dev/null 2>&1; then
  echo "Creating multi-platform buildx builder..."
  docker buildx create --name multiplatform --use
else
  docker buildx use multiplatform
fi

echo "Building and pushing ${IMAGE} for ${PLATFORMS}..."
docker buildx build \
  --platform "${PLATFORMS}" \
  --push \
  -t "${IMAGE}" \
  .

echo "Done. Pushed ${IMAGE} for ${PLATFORMS}."
