# 贡献指南

本文面向参与 HeyBlog 开发的贡献者，涵盖本地环境、代码边界、数据库变更、验证和提交要求。

## 开始之前

仓库使用以下工具，具体版本以对应来源为准：

- Node.js：`.nvmrc`
- pnpm：`package.json#packageManager`
- Go：`.go-version` 和 `go.work`
- Task：仓库与模块命令入口
- Docker Compose：本地 PostgreSQL/AGE 和 Redis
- golangci-lint：`.golangci-lint-version`

修改任何模块前，先阅读仓库根目录及目标模块最近的 `AGENTS.md`。首次检出后执行：

```bash
task setup
```

该命令安装 Node.js 和 Go 依赖、Git hooks，并同步 Web 内容。需要分别执行时使用：

```bash
task install
task prepare
```

不要提交真实凭据、生产数据、本地环境文件或应用生成物。

## 本地开发

复制开发环境模板和 API 配置：

```bash
cp .env.development.example .env.development
cp apps/api/config/conf.development.example.yaml apps/api/config/conf.yaml
```

`.env.development` 同时供 API 和 Web 开发任务使用。默认服务绑定如下：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `API_MIGRATION_DATABASE_URL` | `postgres://migrator:migrator_dev@127.0.0.1:5432/heyblog?sslmode=disable` | Goose 迁移连接 |
| `API_DATABASE_URL` | `postgres://api_runtime:api_runtime_dev@127.0.0.1:5432/heyblog?sslmode=disable` | API 运行时连接池 |
| `API_REDIS_URL` | `redis://127.0.0.1:6379/0` | Redis 连接 |
| `API_HEALTHCHECK_TOKEN` | `development-healthcheck-token-0123456789` | API 健康检查认证 |
| `API_WEB_TOKEN` | `development-web-service-token-0123456789` | Web 到 API 的服务认证 |
| `WEB_API_BASE_URL` | `http://127.0.0.1:10201` | Web SSR 使用的 API 地址 |

默认密码和令牌仅适用于本机开发。需要测试邮件或 GitHub OAuth 时，再在未跟踪的环境文件中填写模板声明的 AWS 和 GitHub 变量。

启动开发依赖：

```bash
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

分别启动 API 和 Web：

```bash
task api:dev
task web:dev
```

Web 默认地址为 `http://127.0.0.1:10101`，API 默认地址为 `http://127.0.0.1:10201`。浏览器数据请求通过 Web 同源端点转发到 API。本地 GitHub OAuth callback 为 `http://127.0.0.1:10101/auth/github/callback`。

停止开发依赖并保留数据：

```bash
docker compose -f infra/docker/docker-compose.env.yaml down
```

开发 Compose 接受以下可选覆盖：

| 变量 | 默认值 |
| --- | --- |
| `POSTGRES_PASSWORD` | `postgres_dev` |
| `POSTGRES_MIGRATOR_PASSWORD` | `migrator_dev` |
| `POSTGRES_RUNTIME_PASSWORD` | `api_runtime_dev` |
| `POSTGRES_HOST_PORT` | `5432` |
| `REDIS_HOST_PORT` | `6379` |

修改端口或密码时同步更新 `.env.development`。角色密码只在 PostgreSQL 卷首次初始化时生效；已有卷不会随环境变量自动更新。

若确认本地 PostgreSQL 数据可以永久删除，可重建开发数据库卷：

```bash
docker compose -f infra/docker/docker-compose.env.yaml down
docker volume rm heyblog-dev-env_postgres_data
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

不要以 root 身份运行 Task。Docker 需要提权时，仅对人工 Docker 命令使用 `sudo -- docker ...`；容器验证任务使用 `DOCKER_COMMAND='sudo -- docker'`。

## 代码与模块边界

- `apps/api` 负责 HTTP API、业务规则、认证授权、数据库、Redis 和外部服务生命周期。
- `apps/web` 负责 Astro SSR、页面、同源接口和浏览器交互，只通过 HTTP 使用 API。
- `packages/node/configs` 负责共享 Node.js 工具配置。
- `infra` 负责开发依赖与基础设施配置。

应用环境变量由各模块专用配置入口读取。新增变量前先确认现有变量不能表达同一语义，并同步更新对应模板和测试。不要在 Web 浏览器代码中暴露服务地址、数据库概念或私密令牌。

`apps/web/contents` 是由远端内容仓库同步生成的快照，不要直接编辑。需要更新时执行：

```bash
task web:prepare
```

新增、删除或升级依赖时，使用所属模块的包管理器更新清单与锁文件；不要手动修改锁文件或生成文件。

## 数据库变更

迁移位于 `apps/api/internal/database/migrations/sql`，sqlc 查询位于 `apps/api/internal/database/queries`。

进行 Schema 或查询变更时：

1. 新增有序 Goose 迁移，并提供对应的 Down 操作。
2. 为每个迁移字段同时添加行内 `--` 注释和 `COMMENT ON COLUMN`。
3. 补齐主键、外键、唯一性、非空约束和查询所需索引。
4. 更新 sqlc 查询并执行 `task api:sqlc:generate`。
5. 执行 `task api:sqlc:vet`、`task api:sqlc:diff` 和 `task api:test:integration`。

不要手动修改 `apps/api/internal/database/gen` 中的 sqlc 生成文件。应用代码只能使用 `migrator` 执行迁移，使用 `api_runtime` 处理运行时请求；不得注入 PostgreSQL 管理员连接。

## 检查与测试

迭代时运行最小相关检查：

```bash
task api:verify
task web:verify
task compose:check
```

提交前运行全部离线检查：

```bash
task verify
```

数据库、Redis 或迁移行为变更还必须运行：

```bash
task api:test:integration
```

依赖、安全行为或容器配置变更且网络可用时运行：

```bash
task verify:full
```

不得删除、禁用或弱化测试和质量门禁来通过检查。任务失败时先处理由当前变更引入的问题，并明确记录无关的既有失败。

## 提交变更

`task setup` 安装的 pre-commit hook 会格式化受支持的暂存文件，commit-msg hook 会检查提交标题。

提交信息使用 Conventional Commits，标题不超过 72 个字符，例如：

```text
feat(api): add site repository
fix(web): preserve upstream status
docs: clarify local setup
```

每个提交保持单一目的，不混入无关格式化、本地配置或生成物。提交前检查暂存差异，并确认相关模块验证及 `task verify` 已通过。

提交到 GitHub 后，确认 CI 中的 `Check`、`API race test`、`API integration test`、`Web test`、`Dependency security` 和 `Container` 检查通过，再请求评审。
