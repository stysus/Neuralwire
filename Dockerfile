# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build arguments. PUBLIC_SITE_URL is consumed at frontend build time (it
# prerenders sitemap.xml / canonical / OG URLs), so it must be passed in at
# build time: docker build --build-arg PUBLIC_SITE_URL=https://example.com .
# Railway: set the variable in the service and pass it via build args.
# ---------------------------------------------------------------------------
ARG PUBLIC_SITE_URL=""

# ---------------------------------------------------------------------------
# Stage 1: build the SvelteKit frontend (adapter-static -> frontend/build)
# ---------------------------------------------------------------------------
FROM node:24-alpine AS frontend-builder

# SvelteKit reads $env/dynamic/public at build/prerender time.
ARG PUBLIC_SITE_URL
ENV PUBLIC_SITE_URL=$PUBLIC_SITE_URL

WORKDIR /app/frontend

# Dependency layer first for build cache reuse.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Source + static assets, then build the static export.
COPY frontend/ ./
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2: compile the Go backend
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/backend

# Cache Go modules before copying source.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/neuralwire-server ./cmd/server

# ---------------------------------------------------------------------------
# Stage 3: minimal runtime image
# ---------------------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata su-exec && \
    addgroup -S neuralwire && adduser -S -G neuralwire neuralwire

WORKDIR /app

# Backend binary + built frontend static files.
COPY --from=backend-builder /out/neuralwire-server /app/neuralwire-server
COPY --from=frontend-builder /app/frontend/build /app/static

# Data directory for SQLite; owned by the non-root user.
RUN mkdir -p /app/data && chown -R neuralwire:neuralwire /app

# NOTE: we intentionally do NOT set `USER neuralwire` here. The entrypoint
# runs as root so it can chown Railway-attached volumes (mounted as root),
# then drops privileges with su-exec before exec'ing the server.

# The app runs as a non-root user, but Railway-attached volumes are mounted
# with root ownership. An entrypoint that chowns the data dir before exec
# fixes the permission mismatch, so SQLite can create its DB file.
COPY --chown=neuralwire:neuralwire docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV STATIC_DIR=/app/static \
    DB_PATH=/app/data/neuralwire.db \
    UPLOAD_DIR=/app/data/uploads \
    PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
