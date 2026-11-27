# syntax=docker/dockerfile:1@sha256:572cebbc2b15e6f84c077742952674889cb63cd5e9b80abdffcbe587b6f19183
FROM heyblog-builder:local AS builder
WORKDIR /workspace
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mise run //apps/worker:build

FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2
WORKDIR /app
RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder --chown=10001:10001 /workspace/apps/worker/dist/heyblog-worker /app/heyblog-worker
LABEL org.opencontainers.image.title="HeyBlog Worker"
USER 10001:10001
HEALTHCHECK --interval=15s --timeout=5s --retries=6 \
    CMD ["/app/heyblog-worker", "--healthcheck"]
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/heyblog-worker"]
