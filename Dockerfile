# ============================================================================
# PsiloConValley — Railway Production Dockerfile
# Uses pre-built chrome-base image for fast deploys.
# Base image: ghcr.io/psiloconvalley/chrome-base:bookworm
# ============================================================================

# ---------- Build Stage ----------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

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
