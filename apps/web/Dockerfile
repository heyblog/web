FROM node:24.15.0-trixie AS builder
WORKDIR /workspace

RUN corepack enable

COPY . .
RUN pnpm install --config.inject-workspace-packages=true --no-frozen-lockfile

RUN pnpm -F @zhblogs/web run build

FROM node:24.15.0-trixie-slim AS runner
WORKDIR /workspace

RUN corepack enable

COPY --from=builder /workspace/apps/web/dist ./apps/web/dist

EXPOSE 9101
CMD ["node", "apps/web/dist/server/index.mjs"]
