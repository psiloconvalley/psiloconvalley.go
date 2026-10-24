# syntax=docker/dockerfile:1.7
# ============================================================================
# psiloconvalley — Railway Production Dockerfile
# - Multi-stage build (Go compiler not shipped in runtime)
# - Health check tuned for actual boot time (<5s observed)
# Base image: ghcr.io/psiloconvalley/chrome-base:bookworm
#
# TODO: Add BuildKit cache mounts once Railway cacheKey syntax is confirmed.
# Standard id=<name> and id=s/${VAR}-<name> both rejected by builder v4.
# ============================================================================

# ---------- Build Stage ----------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
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
