# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM heyblog-builder:local AS builder

WORKDIR /workspace

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mise run //apps/api:build

FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4 AS runner

WORKDIR /app

RUN apt-get update \
    && apt-get upgrade -y \
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
