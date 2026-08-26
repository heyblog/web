# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

FROM heyblog-builder:local AS builder

WORKDIR /workspace

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    task container:api-build

FROM debian:trixie-slim@sha256:d7e12182ce18b85b93007c1dedf31f2d29e01ccf3182cc4017c709b6259bc132 AS runner

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder --chown=10001:10001 /workspace/apps/api/dist/heyblog-api /app/heyblog-api
COPY --from=builder --chown=10001:10001 --chmod=0444 /workspace/apps/api/config/default.yaml /app/config/default.yaml

RUN install -d -m 0750 -o 10001 -g 10001 /var/log/heyblog/api

LABEL org.opencontainers.image.title="HeyBlog API"

USER 10001:10001

EXPOSE 10201

VOLUME ["/var/log/heyblog/api"]

HEALTHCHECK --interval=15s --timeout=5s --retries=6 \
  CMD ["/app/heyblog-api", "--healthcheck"]

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/heyblog-api"]
