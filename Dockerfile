# syntax=docker/dockerfile:1
#
# Multi-stage build for ARTEX. Frontend is built in a Node stage, then
# embedded into the Go binary via go:embed.

# -------------------------------------------------------------- frontend ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# --------------------------------------------------------------- builder ---
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist /src/web/dist
ARG TARGETOS
ARG TARGETARCH
ARG ARTEX_VERSION=dev
RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-amd64}" \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${ARTEX_VERSION}" \
      -o /out/artex-cc ./cmd/artex-cc

# --------------------------------------------------------------- runtime ---
FROM alpine:3.21
LABEL org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.source="https://github.com/SREsAreHumanToo/ARTEX-CC"
RUN apk add --no-cache ca-certificates nodejs npm \
    && npm install -g @anthropic-ai/claude-code@latest \
    && npm cache clean --force
WORKDIR /app
COPY --from=build /out/artex-cc /app/artex-cc
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
COPY LICENSE NOTICE /app/
RUN chmod +x /app/artex-cc /app/docker-entrypoint.sh
VOLUME ["/app/data"]
EXPOSE 9400 8788
ENTRYPOINT ["/app/docker-entrypoint.sh"]
