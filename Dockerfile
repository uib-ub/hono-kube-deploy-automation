#
# Build go project
#
# Stage 1: Building the Go application
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

COPY . .

RUN go version

RUN apk add -u -t build-tools curl git && \
  CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o github-deploy-hono ./cmd/. && \
  apk del build-tools && \
  rm -rf /var/cache/apk/*

#
# Stage 2: Minimal runtime image
#
# Only ships what the binary actually uses at runtime:
#   - ca-certificates: TLS to GitHub + GHCR
#   - git:             shelled out from internal/client/github.go
# Docker daemon access is provided by mounting the host's /var/run/docker.sock
# (see deployment/deploy.yaml and docker-compose.yaml). The Go binary talks to
# that daemon via the moby/moby/client SDK — no docker CLI, no DinD, no kubectl.
FROM alpine:3.20

RUN apk --no-cache add ca-certificates git && \
  git config --system http.postBuffer 524288000 && \
  git config --system core.compression 0 && \
  git config --system http.lowSpeedLimit 1000 && \
  git config --system http.lowSpeedTime 60

WORKDIR /app

COPY --from=go-builder /app/github-deploy-hono /github-deploy-hono
COPY --from=go-builder /app/internal/config/config.yaml /app/internal/config/config.yaml

RUN chmod +x /github-deploy-hono

ENTRYPOINT ["/github-deploy-hono"]
