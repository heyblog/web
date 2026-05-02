FROM node:24.15.0-trixie AS builder
WORKDIR /workspace

RUN corepack enable

COPY . .
RUN pnpm install --frozen-lockfile --config.inject-workspace-packages=true

RUN pnpm -F @zhblogs/api run build

FROM node:24.15.0-trixie-slim AS runner
WORKDIR /workspace

RUN corepack enable

COPY --from=builder /workspace/node_modules ./node_modules
COPY --from=builder /workspace/apps/api/node_modules ./apps/api/node_modules
COPY --from=builder /workspace/apps/api/dist ./apps/api/dist

EXPOSE 9201
CMD ["node", "apps/api/dist/index.mjs"]
