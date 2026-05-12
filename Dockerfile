# ============================================================================
# PsiloConValley — Railway / Production Dockerfile (BuildKit–free)
# ============================================================================

# ---------- Stage 1: Dependency Cache (no --mount) ----------
FROM golang:1.23-bookworm AS deps

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

# ---------- Stage 2: Build (no --mount) ----------
FROM golang:1.23-bookworm AS builder

WORKDIR /src

# Pull cached modules from deps stage
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -buildvcs=false \
      -ldflags="-w -s -extldflags '-static'" \
      -o /out/psiloconvalley \
      ./cmd/psiloconvalley

RUN test -x /out/psiloconvalley && echo "✅ Binary OK"

# ---------- Stage 3: Runtime ----------
FROM debian:bookworm-slim AS runtime

LABEL org.opencontainers.image.title="PsiloConValley" \
      org.opencontainers.image.url="https://psiloconvalley.com"

ENV DEBIAN_FRONTEND=noninteractive \
    TZ=UTC

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates \
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
    apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false; \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*; \
    chromium --version

RUN groupadd --gid 1001 appgroup && \
    useradd \
      --uid 1001 \
      --gid appgroup \
      --shell /usr/sbin/nologin \
      --create-home \
      appuser && \
    mkdir -p \
      /home/appuser/.cache/chromium \
      /home/appuser/.config/chromium \
      /tmp/chromedp && \
    chown -R appuser:appgroup \
      /home/appuser \
      /tmp/chromedp

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

COPY --from=builder --chown=appuser:appuser /out/psiloconvalley /app/psiloconvalley
COPY --chown=appuser:appuser templates/ /app/templates/
COPY --chown=appuser:appuser static/    /app/static/

RUN chmod 555 /app/psiloconvalley

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:${PORT}/healthz || exit 1

USER appuser

EXPOSE 8080

CMD ["/app/psiloconvalley"]
