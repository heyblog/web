FROM node:24.15.0-trixie AS builder
WORKDIR /workspace

RUN corepack enable

COPY . .
RUN pnpm install --frozen-lockfile

RUN pnpm -F @zhblogs/web run build

FROM node:24.15.0-trixie-slim AS runner
WORKDIR /workspace

ENV NODE_ENV=production

COPY --from=builder /workspace/node_modules ./node_modules
COPY --from=builder /workspace/package.json ./package.json
COPY --from=builder /workspace/packages/configs ./packages/configs
COPY --from=builder /workspace/apps/web/package.json ./apps/web/package.json
COPY --from=builder /workspace/apps/web/dist ./apps/web/dist

EXPOSE 9101
CMD ["node", "apps/web/dist/server/index.mjs"]
