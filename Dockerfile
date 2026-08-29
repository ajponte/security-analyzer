# ==========================================
# Stage 1: Build Static Go Binary
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Install build dependencies for static linking and git metadata
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go module dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Copy source tree
COPY . .

# Build statically compiled binary (CGO disabled, stripped debug symbols)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w -extldflags '-static'" -o /out/security-analyzer main.go

# ==========================================
# Stage 2: Runtime Image (Python 3.11 + Semgrep)
# ==========================================
FROM python:3.11-slim-bookworm AS runtime

LABEL org.opencontainers.image.title="ajp-security-analyzer" \
      org.opencontainers.image.description="LLM-based security scanner and Semgrep SAST orchestrator for GitHub Actions" \
      org.opencontainers.image.source="https://github.com/ajponte/security-analyzer" \
      org.opencontainers.image.licenses="MIT"

# Set environment variables for non-interactive execution and Python optimization
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/usr/local/bin:/usr/bin:/bin:${PATH}" \
    SEMGREP_ENABLE_VERSION_CHECK=0

# Install runtime dependencies: Git (required for Semgrep repo discovery), CA certs, Curl, JQ, Bash
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    curl \
    jq \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Install Semgrep CLI via pip into global Python environment
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-cache-dir --upgrade pip setuptools && \
    pip install --no-cache-dir semgrep==1.175.0

# Configure global git safe directory to avoid permissions mismatch in GHA workspace mounts
RUN git config --global --add safe.directory '*'

# Copy statically linked binary from builder stage
COPY --from=builder /out/security-analyzer /usr/local/bin/security-analyzer
RUN chmod +x /usr/local/bin/security-analyzer

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default working directory mapped by GitHub Actions
WORKDIR /github/workspace

ENTRYPOINT ["/entrypoint.sh"]
