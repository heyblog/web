FROM node:24.15.0-trixie AS builder
WORKDIR /workspace

RUN corepack enable

COPY . .
RUN pnpm install --frozen-lockfile

RUN pnpm -F @zhblogs/api run build

FROM node:24.15.0-trixie-slim AS runner
WORKDIR /workspace

ENV NODE_ENV=production

COPY --from=builder /workspace/node_modules ./node_modules
COPY --from=builder /workspace/package.json ./package.json
COPY --from=builder /workspace/packages ./packages
COPY --from=builder /workspace/apps/api/package.json ./apps/api/package.json
COPY --from=builder /workspace/apps/api/dist ./apps/api/dist

EXPOSE 9201
CMD ["node", "apps/api/dist/index.mjs"]
