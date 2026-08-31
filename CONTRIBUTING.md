# 贡献指南

## 工具与安装

本仓库使用以下工具：

- Node.js：版本以 `.nvmrc` 为准。
- pnpm：版本以根目录 `package.json#packageManager` 为准。
- Go：工具链版本以 `.go-version` 为准，`go.work` 定义工作区模块。
- Task：仓库和模块命令入口。
- Docker Compose：启动 PostgreSQL/AGE 和 Redis。
- golangci-lint：版本以 `.golangci-lint-version` 为准。
- govulncheck：版本由根目录 `go.mod` 管理，无需全局安装。
- pnpm audit：由固定版本的 pnpm 提供，无需额外安装。

首次检出后执行：

```bash
task setup
```

`task setup` 安装 Node.js 和 Go 依赖（包括根模块声明的 Go 工具）、Git hooks，并同步 Web
内容。需要单独控制副作用时，`task install` 只安装依赖，`task prepare` 只同步内容。工具链
本身需要提前安装。完成后可直接运行 `task verify:full`；该命令使用 pnpm audit
和 govulncheck 扫描 Node.js、Go 依赖，漏洞数据库扫描需要网络连接。

## 开发环境

复制统一的开发环境模板：

```bash
cp .env.development.example .env.development
cp apps/api/config/conf.development.example.yaml apps/api/config/conf.yaml
```

该文件同时供 API 和 Web 开发任务使用，只包含应用必需的服务连接和内部令牌：

| 模块 | 变量 | 默认开发值 | 用途 |
| --- | --- | --- | --- |
| `apps/api` | `API_MIGRATION_DATABASE_URL` | `postgres://migrator:migrator_dev@127.0.0.1:5432/heyblog?sslmode=disable` | Goose 迁移连接 |
| `apps/api` | `API_DATABASE_URL` | `postgres://api_runtime:api_runtime_dev@127.0.0.1:5432/heyblog?sslmode=disable` | API 运行时连接池 |
| `apps/api` | `API_REDIS_URL` | `redis://127.0.0.1:6379/0` | Redis 连接 |
| `apps/api` | `API_HEALTHCHECK_TOKEN` | `development-healthcheck-token-0123456789` | internal live/ready Bearer 认证 |
| API-Web 共享边界 | `API_WEB_TOKEN` | `development-web-service-token-0123456789` | Web 到 API 的服务认证 |
| `apps/web` | `WEB_API_BASE_URL` | `http://127.0.0.1:10201` | Web SSR 使用的私有 API 地址 |

不要提交 `.env.development`、真实凭据或生产数据。默认密码仅用于本机开发。

API 会主动读取固定名称的 `config/default.yaml` 和 `config/conf.yaml`。默认文件保存公共非敏感配置；被忽略的 `conf.yaml` 必须显式声明 `mode: development` 或 `mode: production`。开发任务从 `apps/api` 运行，因此使用该模块下的 `config/`。

三个外部服务 URL、健康检查令牌和 Web 服务令牌由环境变量提供；监听地址、端口、连接池、超时、日志、CORS、代理和健康检查策略由 YAML 管理，不接受对应环境变量或命令行覆盖。开发默认端口为 `10201`。Web 通过 `WEB_API_BASE_URL` 读取 API 地址，所有浏览器数据请求必须进入 Web 同源端点。

开发 Compose 还允许从当前 shell 覆盖以下值：

| 变量 | 默认值 |
| --- | --- |
| `POSTGRES_PASSWORD` | `postgres_dev` |
| `POSTGRES_MIGRATOR_PASSWORD` | `migrator_dev` |
| `POSTGRES_RUNTIME_PASSWORD` | `api_runtime_dev` |
| `POSTGRES_HOST_PORT` | `5432` |
| `REDIS_HOST_PORT` | `6379` |

覆盖 Compose 端口或密码时，需要同步修改 `.env.development` 中对应的 URL。数据库密码初始化脚本只在首次创建 PostgreSQL 卷时执行；已有卷不会因环境变量变化自动更新。

开发数据库使用三个独立角色：

| 角色 | 默认开发密码 | 用途 |
| --- | --- | --- |
| `postgres` | `postgres_dev` | 安装扩展、管理角色和数据库参数等集群级操作 |
| `migrator` | `migrator_dev` | Goose、业务 Schema、日常数据和结构管理 |
| `api_runtime` | `api_runtime_dev` | API 最小运行权限，不用于人工维护 |

数据库软件日常连接使用 `migrator`；只有集群级管理才使用 `postgres`。生产环境不得把 `postgres` 连接串注入 API，并应通过受控运维网络、容器内连接或 SSH 隧道使用管理员账户。

## 启动与停止

启动 PostgreSQL 18 + AGE 1.7.0 和 Redis 8：

```bash
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

分别在两个终端启动应用：

```bash
task api:dev
task web:dev
```

停止开发依赖但保留数据卷：

```bash
docker compose -f infra/docker/docker-compose.env.yaml down
```

如确认本地 PostgreSQL 数据可以全部删除，可只重建 PostgreSQL 卷并保留 Redis：

```bash
docker compose -f infra/docker/docker-compose.env.yaml down
docker volume rm heyblog-dev-env_postgres_data
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

删除 `heyblog-dev-env_postgres_data` 会永久删除本地 PostgreSQL 数据库；Redis 数据卷不受影响。
Task 只提供 `task compose:check` 配置校验，不管理这些服务的生命周期。Docker 需要提权时，
对人工 Docker/Compose 命令使用 `sudo -- docker ...`；对容器 Task 使用
`task container:verify DOCKER_COMMAND='sudo -- docker'`。rootless Docker 可通过 Unix socket 形式的
`DOCKER_HOST` 选择 daemon；容器扫描会将该 socket 映射给 Trivy。禁止使用 `sudo task`。

## 生产容器

生产镜像内的 API 位于 `/app/heyblog-api`，配置目录为 `/app/config`。部署时复制 `.env.production.example` 为 `.env.production` 并替换全部占位值，同时提供 `/app/config/conf.yaml`，可从 `apps/api/config/conf.production.example.yaml` 开始。通过 `docker --env-file .env.production` 或 Compose `env_file` 注入外部服务 URL、健康检查令牌和 Web 服务令牌。Web 容器使用同一 `API_WEB_TOKEN`，并通过 `WEB_API_BASE_URL` 中配置的私有 API 服务地址访问 API；模板不假定具体的生产服务 DNS。

API 和 Web 必须位于同一私有容器网络，只发布 Web 的 `9101` 端口。API 的 `10201` 仅在内部网络开放，不配置主机端口映射。入口代理也只能转发到 Web，不能为 API 创建公网路由。

运行 `task container:scan` 可检查已经构建的 `heyblog-api:local` 和 `heyblog-web:local` 镜像；
`task container:publish REGISTRY=... NAMESPACE=... IMAGE_TAGS='sha,latest'` 只标记并推送已经由
`task container:verify` 验证的本地镜像，不会隐式构建或扫描；这是远程写操作，三个参数均为
必填。CI 总是先推送不可变的提交 SHA；仅当远端 main 仍指向当前提交时才更新 `latest`。
两个镜像及两个标签的更新不是镜像仓库级原子操作。

API 默认无参数启动。`--healthcheck` 是唯一支持的参数，供 Docker 使用配置中的 Bearer 令牌请求 `/health/ready`；其他参数均会被拒绝。API `/ping` 是 web-internal 端点，缺少有效 `X-HeyBlog-Web-Token` 时返回 401。Web `/api/ping` 仅接受同源页面 fetch，地址栏导航、跨站请求或缺少 Fetch Metadata 的请求返回 403。`/health/live` 和 `/health/ready` 是 internal 端点。

未来第三方接口优先在 Web 中声明为 `public`，逐路由配置 CORS、方法和上游字段。若需要绕过 Web 直接开放 Go API，应将对应 Go 路由显式注册为 public 并单独调整入口网络；不得修改 web-internal 认证的全局默认值。

## 模块边界

- `apps/api` 拥有业务规则、认证授权、数据库 Schema、迁移、查询和连接生命周期。
- `apps/web` 通过 HTTP 使用 API，不直接访问数据库，也不暴露内部服务地址。
- 共享代码只有在存在真实跨模块需求时才进入 `packages`。
- `apps/web/contents` 是同步生成的提交固定快照，不要直接编辑；需要更新时执行 `task web:prepare`。

修改模块前应先阅读仓库根目录和目标模块最近的 `AGENTS.md`。

## 数据库变更

数据库定义位于 `apps/api/internal/database/migrations/sql`，查询位于 `apps/api/internal/database/queries`。

进行 Schema 或查询变更时：

1. 新增有序 Goose 迁移，并提供对应的 Down 操作。
2. 为每个迁移字段同时添加行内 `--` 注释和 `COMMENT ON COLUMN`。
3. 补齐主键、外键、唯一性、非空和查询所需索引。
4. 更新 sqlc 查询并执行 `task api:sqlc:generate`。
5. 执行 `task api:sqlc:vet`、`task api:sqlc:diff` 和 `task api:test:integration`。

不要手动修改 `internal/database/gen` 中的 sqlc 生成文件。

## 检查与测试

迭代时优先运行最小相关检查：

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

依赖或安全行为变更且网络可用时运行：

```bash
task verify:full
```

不得删除、禁用或弱化测试和质量门禁来通过检查。

## Git 与提交

`task setup` 会安装仓库 Git hooks。Pre-commit hook 会格式化暂存的受支持文件，commit-msg hook 会校验提交信息。

GitHub 分支保护应使用统一 CI 的 `Check`、`API race test`、`API integration test`、`Web test`、
`Dependency security` 和 `Container` 检查名称；仓库管理员需在工作流切换后手动更新 required checks。

提交信息使用 Conventional Commits，标题不超过 72 个字符，例如：

```text
feat(api): add site repository
fix(web): preserve upstream status
docs: add development setup
```

每个变更应保持单一目的，不包含无关格式化、生成文件或本地环境文件。提交前确认相关模块检查、`task verify` 和必要的集成测试通过。
