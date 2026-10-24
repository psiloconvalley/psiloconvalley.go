# syntax=docker/dockerfile:1.7
# ============================================================================
# psiloconvalley — Railway Production Dockerfile
# Optimized for Docker layer caching + BuildKit cache mounts:
# - Go module cache persists between builds (saves ~5-10s)
# - Go build cache persists between builds (saves ~10-20s)
# - Template/static/migration changes do NOT rebuild binary
# - Health check tuned for actual boot time (<5s observed)
# Base image: ghcr.io/psiloconvalley/chrome-base:bookworm
# ============================================================================

# ---------- Build Stage ----------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Go module layer — cached unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy only Go source needed to build the binary
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# BuildKit cache mounts persist the Go build cache across builds.
# First deploy: no benefit. Every subsequent deploy: 10-20s faster.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -buildvcs=false \
      -ldflags="-w -s" \
      -o /out/psiloconvalley \
      ./cmd/psiloconvalley

# ---------- Runtime Stage ----------
FROM ghcr.io/psiloconvalley/chrome-base:bookworm

COPY --from=builder --chown=appuser:appgroup /out/psiloconvalley /app/psiloconvalley
COPY --chown=appuser:appgroup templates/ /app/templates/
COPY --chown=appuser:appgroup static/ /app/static/
COPY --chown=appuser:appgroup migrations/ /app/migrations/

RUN chmod 555 /app/psiloconvalley

USER appuser

EXPOSE 8080

# App boots in <5s. Previous 45s start-period was 10x overkill
# and directly delayed when Railway marked deploys as "live".
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD curl -fsS http://127.0.0.1:${PORT}/healthz || exit 1

CMD ["/app/psiloconvalley"]
