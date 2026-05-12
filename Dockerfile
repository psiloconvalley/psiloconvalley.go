# ============================================================================
# PsiloConValley — Railway Production Dockerfile
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
FROM debian:bookworm-slim AS runtime

LABEL org.opencontainers.image.title="PsiloConValley" \
      org.opencontainers.image.url="https://psiloconvalley.com"

ENV DEBIAN_FRONTEND=noninteractive \
    TZ=UTC

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      tzdata \
      chromium \
      chromium-sandbox \
      fonts-liberation \
      fonts-noto-cjk \
      fonts-noto-color-emoji \
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
    ; \
    rm -rf /var/lib/apt/lists/*

RUN groupadd --system --gid 1001 appgroup && \
    useradd \
      --system \
      --uid 1001 \
      --gid appgroup \
      --create-home \
      --home-dir /home/appuser \
      --shell /usr/sbin/nologin \
      appuser && \
    mkdir -p \
      /app \
      /tmp/chromedp \
      /home/appuser/.cache/chromium \
      /home/appuser/.config/chromium && \
    chown -R appuser:appgroup /app /tmp/chromedp /home/appuser

WORKDIR /app

ENV HOME=/home/appuser \
    XDG_CACHE_HOME=/home/appuser/.cache \
    XDG_CONFIG_HOME=/home/appuser/.config \
    CHROME_BIN=/usr/bin/chromium \
    CHROMIUM_PATH=/usr/bin/chromium \
    CHROMEDP_NO_SANDBOX=true \
    PORT=8080 \
    GOGC=100 \
    GOMEMLIMIT=512MiB

COPY --from=builder --chown=appuser:appgroup /out/psiloconvalley /app/psiloconvalley
COPY --chown=appuser:appgroup templates/ /app/templates/
COPY --chown=appuser:appgroup static/ /app/static/

RUN chmod 555 /app/psiloconvalley

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=45s --retries=5 \
    CMD curl -fsS http://127.0.0.1:${PORT}/healthz || exit 1

CMD ["/app/psiloconvalley"]
