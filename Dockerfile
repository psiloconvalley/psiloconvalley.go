# ============================================================================
# psiloconvalley — Railway Production Dockerfile
# Optimized for Docker layer caching:
# - Go source changes rebuild binary
# - template/static/migration changes do NOT rebuild binary
# Base image: ghcr.io/psiloconvalley/chrome-base:bookworm
# ============================================================================

# ---------- Build Stage ----------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Go module cache layer
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy only Go source needed to build the binary
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

HEALTHCHECK --interval=15s --timeout=5s --start-period=45s --retries=5 \
    CMD curl -fsS http://127.0.0.1:${PORT}/healthz || exit 1

CMD ["/app/psiloconvalley"]
