# ============================================================================
# PsiloConValley — Production Dockerfile
# Multi-stage, hardened, optimized for Railway / any container platform
# ============================================================================

# ---------- Stage 1: Dependency Cache ----------
# Separate stage so deps only re-download when go.mod/go.sum change
FROM golang:1.23-bookworm AS deps

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# ---------- Stage 2: Build ----------
FROM golang:1.23-bookworm AS builder

WORKDIR /src

# Pull cached modules from previous stage
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY --from=deps /src/go.mod /src/go.sum ./

COPY . .

# Build with every optimization flag that matters
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -buildvcs=false \
      -ldflags="-w -s -extldflags '-static'" \
      -o /out/psiloconvalley \
      ./cmd/psiloconvalley

# Quick sanity check — binary exists and is executable
RUN test -x /out/psiloconvalley && echo "✅ Binary OK"

# ---------- Stage 3: Runtime ----------
FROM debian:bookworm-slim AS runtime

# Metadata
LABEL org.opencontainers.image.title="PsiloConValley" \
      org.opencontainers.image.url="https://psiloconvalley.com" \
      org.opencontainers.image.source="https://github.com/your-org/psiloconvalley"

# ---- System Setup ----
ENV DEBIAN_FRONTEND=noninteractive \
    TZ=UTC

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      # TLS & time
      ca-certificates \
      tzdata \
      # Chromium for PDF generation
      chromium \
      chromium-sandbox \
      # Fonts — covers Latin, CJK, emoji
      fonts-liberation \
      fonts-noto-cjk \
      fonts-noto-color-emoji \
      # Needed by Chromium in containers
      libnss3 \
      libatk1.0-0 \
      libatk-bridge2.0-0 \
      libcups2 \
      libdrm2 \
      libxcomposite1 \
      libxdamage1 \
      libxrandr2 \
      libgbm1 \
      libpango-1.0-0 \
      libcairo2 \
      libasound2 \
      libxshmfence1 \
      libx11-xcb1 \
      # Networking debug (optional, tiny)
      curl \
    ; \
    # Cleanup
    apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false; \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*; \
    # Verify Chromium installed
    chromium --version

# ---- Non-Root User ----
RUN groupadd --gid 1001 appgroup && \
    useradd \
      --uid 1001 \
      --gid appgroup \
      --shell /usr/sbin/nologin \
      --create-home \
      appuser && \
    # Chromium needs writable dirs
    mkdir -p \
      /home/appuser/.cache/chromium \
      /home/appuser/.config/chromium \
      /tmp/chromedp && \
    chown -R appuser:appgroup \
      /home/appuser \
      /tmp/chromedp

WORKDIR /app

# ---- Environment ----
ENV HOME=/home/appuser \
    XDG_CACHE_HOME=/home/appuser/.cache \
    XDG_CONFIG_HOME=/home/appuser/.config \
    # Chromium
    CHROME_BIN=/usr/bin/chromium \
    CHROMIUM_PATH=/usr/bin/chromium \
    CHROMEDP_NO_SANDBOX=true \
    # Go app
    PORT=8080 \
    # Hardening
    GOGC=100 \
    GOMEMLIMIT=512MiB

# ---- Copy Artifacts ----
# Binary
COPY --from=builder --chown=appuser:appgroup /out/psiloconvalley /app/psiloconvalley

# Templates & static assets
COPY --chown=appuser:appgroup templates/ /app/templates/
COPY --chown=appuser:appgroup static/    /app/static/

# ---- Security: read-only binary ----
RUN chmod 555 /app/psiloconvalley

# ---- Healthcheck ----
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:${PORT}/healthz || exit 1
# ---- Drop to non-root ----
USER appuser

EXPOSE 8080

# ---- Entrypoint ----
CMD ["/app/psiloconvalley"]
