# syntax=docker/dockerfile:1.6
FROM golang:alpine AS builder

RUN apk --no-cache add git

WORKDIR /go/src/matterbridge

# Dependency manifests first — cached unless go.mod/go.sum change.
COPY go.mod go.sum ./

# Vendor tree next — cached unless vendored deps change.
COPY vendor/ vendor/

# Source last — most frequently changed.
COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GITHASH="$(git log --pretty=format:'%h' -n 1 2>/dev/null || echo unknown)" \
    && BUILDTIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    && CGO_ENABLED=0 go build -mod vendor \
        -ldflags "-X github.com/42wim/matterbridge/version.GitHash=${GITHASH} -X github.com/42wim/matterbridge/version.BuildTime=${BUILDTIME}" \
        -o /bin/matterbridge

FROM alpine
RUN apk --no-cache add ca-certificates mailcap \
    && mkdir /etc/matterbridge \
    && touch /etc/matterbridge/matterbridge.toml \
    && ln -sf /matterbridge.toml /etc/matterbridge/matterbridge.toml
COPY --from=builder /bin/matterbridge /bin/matterbridge
ENTRYPOINT ["/bin/matterbridge", "-conf", "/etc/matterbridge/matterbridge.toml"]
