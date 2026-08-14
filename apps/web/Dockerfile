# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

FROM heyblog-builder:local AS builder

WORKDIR /workspace

COPY . .

RUN --mount=type=cache,id=heyblog-pnpm-store,target=/root/.local/share/pnpm/store \
    task container:web-build

FROM node:24.19.0-trixie-slim@sha256:0711b541c1c33a8a530ac4f0d391baa9a15b3d804695b1b24a47daa5fb60e74d AS runner

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends tini \
    && rm -rf /var/lib/apt/lists/*

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=9101

LABEL org.opencontainers.image.title="HeyBlog Web"

COPY --from=builder --chown=node:node /workspace/apps/web/dist ./dist

USER node

EXPOSE 9101

HEALTHCHECK --interval=15s --timeout=5s --retries=6 \
  CMD node -e "fetch('http://127.0.0.1:' + process.env.PORT).then((response) => process.exit(response.ok ? 0 : 1)).catch(() => process.exit(1))"

STOPSIGNAL SIGTERM

ENTRYPOINT ["tini", "-e", "143", "--", "node", "dist/server/entry.mjs"]
