#
# Build go project
#
# Stage 1: Building the Go application
FROM golang:1.22-alpine as go-builder

WORKDIR /app

COPY . .

RUN go version

RUN apk add -u -t build-tools curl git && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o github-deploy-hono ./cmd/. && \
    apk del build-tools && \
    rm -rf /var/cache/apk/* 

#
# Stage 2: Setup the runtime environment
# 
FROM docker:19.03.12-dind

# Install system dependencies
RUN apk --no-cache add ca-certificates bash curl git

# Install kubectl from the official source
RUN curl -LO "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl

WORKDIR /app

COPY --from=go-builder /app/github-deploy-hono /github-deploy-hono
COPY --from=go-builder /app/internal/config/config.yaml /app/internal/config/config.yaml

# Make sure your Go application is executable
RUN chmod +x /github-deploy-hono

# Run Docker daemon entrypoint and then your application
ENTRYPOINT ["dockerd-entrypoint.sh"]
CMD ["/github-deploy-hono"]