# ── Stage 1: Build the Go binary ──────────────────────────────────────
FROM golang:1.26-bookworm AS builder

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy full source
COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o psiloconvalley .

# ── Stage 2: Runtime with Chromium ────────────────────────────────────
FROM debian:bookworm-slim

# Install Chromium and all required dependencies
RUN apt-get update && apt-get install -y \
    chromium \
    chromium-sandbox \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-noto-cjk \
    ca-certificates \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the compiled binary from builder stage
COPY --from=builder /app/psiloconvalley .

# Tell chromedp where Chromium lives
ENV CHROMIUM_PATH=/usr/bin/chromium
ENV CHROME_PATH=/usr/bin/chromium

# Railway injects PORT — default to 8080
ENV PORT=8080

# Run as non-root for security
RUN useradd -m -u 1001 appuser
USER appuser

EXPOSE 8080

CMD ["./psiloconvalley"]
