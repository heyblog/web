# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM heyblog-builder:local AS builder

ARG WEB_BUILD_COMMIT
ARG WEB_BUILD_REF
ARG WEB_BUILD_COMMIT_TIME
ARG WEB_BUILD_REPOSITORY_URL
ARG WEB_BUILD_TIME

WORKDIR /workspace

COPY . .

RUN --mount=type=cache,id=heyblog-pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile \
    && WEB_BUILD_COMMIT="${WEB_BUILD_COMMIT}" \
       WEB_BUILD_REF="${WEB_BUILD_REF}" \
       WEB_BUILD_COMMIT_TIME="${WEB_BUILD_COMMIT_TIME}" \
       WEB_BUILD_REPOSITORY_URL="${WEB_BUILD_REPOSITORY_URL}" \
       WEB_BUILD_TIME="${WEB_BUILD_TIME}" \
       mise run //apps/web:build

FROM node:24.21.0-trixie-slim@sha256:8ec5d7557396cfe32d21c3f9c13072355ceab22b584578ca4bb28af31120cffe AS runner

WORKDIR /app

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends openssl tini \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /opt/yarn-v1.22.22 /usr/local/lib/node_modules/npm \
    && rm -f /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/yarn /usr/local/bin/yarnpkg

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=10101

LABEL org.opencontainers.image.title="HeyBlog Web"

COPY --from=builder --chown=node:node /workspace/apps/web/dist ./dist

USER node

EXPOSE 10101

HEALTHCHECK --interval=15s --timeout=5s --retries=6 \
  CMD node -e "fetch('http://127.0.0.1:' + process.env.PORT).then((response) => process.exit(response.ok ? 0 : 1)).catch(() => process.exit(1))"

STOPSIGNAL SIGTERM

ENTRYPOINT ["tini", "-e", "143", "--", "node", "dist/server/entry.mjs"]
