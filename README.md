# HeyBlog

HeyBlog 是一个收集、整理并链接个人博客站点的工作区。后端由 Go API 负责业务、数据库与权限，前端由 Astro 和 Svelte 提供服务端渲染页面与交互界面。

## 工作区

- `apps/api`：Gin API、PostgreSQL/AGE 数据库迁移、sqlc 查询和 Redis 接入。
- `apps/web`：Astro SSR 前端和 Svelte 交互组件。
- `packages/node/configs`：共享 Node.js 工具配置。
- `infra`：开发依赖和部署基础设施。

## 快速开始

准备好 Node.js、pnpm、Go、Task 和 Docker Compose 后执行：

```bash
task setup
cp .env.development.example .env.development
cp apps/api/config/conf.development.example.yaml apps/api/config/conf.yaml
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

分别启动 API 和 Web：

```bash
task api:dev
task web:dev
```

开发依赖的启停由开发者直接使用 Docker Compose 管理；如果本机 Docker 需要提权，在上述
Docker 命令前使用 `sudo --`，不要以 root 身份运行 Task。

Web 默认访问 `http://127.0.0.1:10101`。浏览器数据请求通过 Web 同源 `/api/*` 端点转发，
不直接访问 Go API。API 就绪检查为
`http://127.0.0.1:10201/health/ready`，请求必须携带
`Authorization: Bearer <API_HEALTHCHECK_TOKEN>`。

完整的环境变量、开发流程、数据库变更和提交要求参见 [贡献指南](./CONTRIBUTING.md)。
