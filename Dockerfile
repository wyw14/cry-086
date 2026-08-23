# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS go-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && addgroup -S crane && adduser -S -G crane -u 10001 crane
WORKDIR /app
COPY --from=go-builder /out/server /app/server
COPY --from=go-builder /out/migrate /app/migrate
COPY --from=go-builder /src/web/dist /app/web/dist
COPY migrations /app/migrations
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 0555 /app/server /app/migrate /app/docker-entrypoint.sh && mkdir -p /app/var/files && chown -R crane:crane /app
USER crane
EXPOSE 8080
ENV LISTEN_ADDRESS=:8080 WEB_DIST=/app/web/dist FILE_ROOT=/app/var/files
ENTRYPOINT ["/app/docker-entrypoint.sh"]
