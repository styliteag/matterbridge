# Build

## Docker

### Prerequisites

Ensure you have a buildx builder that supports multi-platform builds:

```bash
docker buildx create --use
```

### Build and push (multi-arch)

Builds for both `linux/amd64` and `linux/arm64` and pushes a multi-arch manifest to Docker Hub:

```bash
docker buildx build --platform linux/amd64,linux/arm64 --push -t styliteag/matterbridge2 .
```

> **Note:** Multi-platform builds require `--push` — you cannot `--load` to the local daemon with multiple platforms.

### Build for a single platform

To build for amd64 only:

```bash
docker buildx build --platform linux/amd64 --push -t styliteag/matterbridge2 .
```

To load a single-platform image locally (without pushing):

```bash
docker buildx build --platform linux/amd64 --load -t styliteag/matterbridge2 .
```

### Run

```bash
docker run -v /path/to/matterbridge.toml:/matterbridge.toml styliteag/matterbridge2
```

### Gotcha: Apple Silicon

Building on an Apple Silicon Mac without `--platform` produces an `arm64`-only image. Pulling that image on an `amd64` host fails with:

```
no matching manifest for linux/amd64 in the manifest list entries
```

Always specify `--platform` explicitly when building for deployment.
