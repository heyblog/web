# 贡献指南

本文面向参与 HeyBlog 开发的贡献者，涵盖本地环境、代码边界、数据库变更、验证和提交要求。

## 开始之前

仓库使用 mise 管理工具版本和命令。安装满足 `mise.toml#min_version` 的 mise 后，工具版本以
`mise.toml` 和 `mise.lock` 为准：

- Node.js、pnpm、Go、golangci-lint 和 ShellCheck：根目录 `mise.toml`
- mise：根目录 `mise.toml#min_version`
- Docker Compose：本地 PostgreSQL/AGE 和 Redis

修改任何模块前，先阅读仓库根目录及目标模块最近的 `AGENTS.md`。首次检出后执行：

```bash
mise install --locked
mise run setup
```

该命令安装 Node.js 和 Go 依赖、Git hooks，并同步 Web 内容。需要分别执行时使用：

```bash
mise run install
mise run prepare
```

不要提交真实凭据、生产数据、本地环境文件或应用生成物。

## 项目版本

根目录 `mise.toml` 的 `[vars].project_version` 是 API、Web 和仓库内部 package 的唯一项目
版本来源。各模块的 `package.json` 不单独声明 `version`。使用以下命令查看、校验或更新版本：

```bash
mise run version:show
mise run version:check
mise run version:set -- 0.1.5
mise run version:patch
mise run version:minor
mise run version:major
```

版本只接受不带前导零、预发布或构建后缀的 `X.Y.Z`，且每段必须是 JavaScript 安全非负整数。
递增采用 SemVer 核心版本语义：patch 增加 Z；minor 增加 Y 并将 Z 归零；major 增加 X 并将
Y、Z 归零。命令只修改 `mise.toml` 中的变量，不会创建 Git 标签、提交或发布产物。

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
| `API_MAIL_SMTP_URL` | `smtp://127.0.0.1:1025` | Mailpit SMTP 捕获服务 |
| `API_HEALTHCHECK_TOKEN` | `development-healthcheck-token-0123456789` | API 健康检查认证 |
| `API_WEB_TOKEN` | `development-web-service-token-0123456789` | Web 到 API 的服务认证 |
| `WEB_API_BASE_URL` | `http://127.0.0.1:10201` | Web SSR 使用的 API 地址 |

默认密码和令牌仅适用于本机开发。开发邮件默认由 Mailpit 捕获，无需 AWS 凭据；GitHub OAuth
仍需在未跟踪的环境文件中填写 GitHub 变量。

启动开发依赖：

```bash
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

分别启动 API 和 Web：

```bash
mise run //apps/api:dev
mise run //apps/web:dev
```

Web 默认地址为 `http://127.0.0.1:10101`，API 默认地址为 `http://127.0.0.1:10201`。浏览器数据请求通过 Web 同源端点转发到 API。本地 GitHub OAuth callback 为 `http://127.0.0.1:10101/auth/github/callback`。

Mailpit 收件箱位于 `http://127.0.0.1:8025`。注册、重新发送验证码、找回密码和站点审核
通知都会进入该收件箱；可使用 `developer@example.test` 等本地地址完成完整认证流程。需要直接
测试 AWS SES 时，在 `config/conf.yaml` 中设置 `mail.transport: ses`，并通过 AWS SDK 标准环境
变量提供凭据。

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
| `MAILPIT_SMTP_HOST_PORT` | `1025` |
| `MAILPIT_UI_HOST_PORT` | `8025` |

修改数据库、Redis 或 Mailpit SMTP 端口时同步更新 `.env.development`。Mailpit UI 端口只影响浏览器访问。角色密码只在 PostgreSQL 卷首次初始化时生效；已有卷不会随环境变量自动更新。

若确认本地 PostgreSQL 数据可以永久删除，可重建开发数据库卷：

```bash
docker compose -f infra/docker/docker-compose.env.yaml down
docker volume rm heyblog-dev-env_postgres_data
docker compose -f infra/docker/docker-compose.env.yaml up -d --wait
```

不要以 root 身份运行 mise。Docker 需要提权时，仅对人工 Docker 命令使用 `sudo -- docker ...`；容器验证任务使用 `DOCKER_COMMAND='sudo -- docker' mise run container:verify`。

## 代码与模块边界

- `apps/api` 负责 HTTP API、业务规则、认证授权、数据库、Redis 和外部服务生命周期。
- `apps/web` 负责 Astro SSR、页面、同源接口和浏览器交互，只通过 HTTP 使用 API。
- `packages/node/configs` 负责共享 Node.js 工具配置。
- `infra` 负责开发依赖与基础设施配置。

应用环境变量由各模块专用配置入口读取。新增变量前先确认现有变量不能表达同一语义，并同步更新对应模板和测试。不要在 Web 浏览器代码中暴露服务地址、数据库概念或私密令牌。
API 和 Web 必须位于同一私有容器网络，只发布 Web 的 `10101` 端口。API 的 `10201` 仅在内部网络开放，不配置主机端口映射。入口代理也只能转发到 Web，不能为 API 创建公网路由。

`apps/web/contents` 是由远端内容仓库同步生成的快照，不要直接编辑。需要更新时执行：

```bash
mise run //apps/web:prepare
```

新增、删除或升级依赖时，使用所属模块的包管理器更新清单与锁文件；不要手动修改锁文件或生成文件。

## 数据库变更

迁移位于 `apps/api/internal/infrastructure/database/migrations/sql`，sqlc 查询位于 `apps/api/internal/infrastructure/database/queries`。

进行 Schema 或查询变更时：

1. 新增有序 Goose 迁移，并提供对应的 Down 操作。
2. 为每个迁移字段同时添加行内 `--` 注释和 `COMMENT ON COLUMN`。
3. 补齐主键、外键、唯一性、非空约束和查询所需索引。
4. 更新 sqlc 查询并执行 `mise run //apps/api:sqlc:generate`。
5. 执行 `mise run //apps/api:sqlc:vet`、`mise run //apps/api:sqlc:diff` 和
   `mise run //apps/api:test:integration`。

不要手动修改 `apps/api/internal/infrastructure/database/gen` 中的 sqlc 生成文件。应用代码只能使用 `migrator` 执行迁移，使用 `api_runtime` 处理运行时请求；不得注入 PostgreSQL 管理员连接。

## 检查与测试

迭代时运行最小相关检查：

```bash
mise run //apps/api:verify
mise run //apps/web:verify
mise run compose:check
```

提交前运行全部离线检查：

```bash
mise run verify
```

数据库、Redis 或迁移行为变更还必须运行：

```bash
mise run //apps/api:test:integration
```

依赖、安全行为或容器配置变更且网络可用时运行：

```bash
mise run verify:full
```

不得删除、禁用或弱化测试和质量门禁来通过检查。任务失败时先处理由当前变更引入的问题，并明确记录无关的既有失败。

工作流检查使用 mise 管理的 ShellCheck，确保本地与 CI 执行相同的 Shell 检查。
CI 分别缓存工具安装、pnpm store 和 Go 模块及编译产物；工具缓存不能替代依赖缓存。

## 提交变更

`mise run setup` 安装的 pre-commit hook 会格式化受支持的暂存文件，commit-msg hook 会检查提交标题。

提交信息使用 Conventional Commits，标题不超过 72 个字符，例如：

```text
feat(api): add site repository
fix(web): preserve upstream status
docs: clarify local setup
```

每个提交保持单一目的，不混入无关格式化、本地配置或生成物。提交前检查暂存差异，并确认相关模块验证及 `mise run verify` 已通过。

提交 PR 到 GitHub 后，确认 `CI gate`、CodeQL 和 `dependency-review` 检查通过，再请求评审。`CI gate` 汇总内容准备、静态检查、API race/integration、Web test 和依赖安全检查，任何失败、取消或跳过都会使门禁失败。增量依赖审查会阻止引入高危及以上漏洞的依赖变更。

CI 中的 `Dependency security` 每次运行全量依赖安全扫描。作者为 `renovate[bot]` 的 PR、main push 和所有手动 CI 运行保持严格检查，扫描失败会使 job 和 `CI gate` 失败。其他 PR 的扫描失败仍显示告警及日志入口，但不阻断 CI；依赖安装等前置步骤失败仍会阻断。

独立的 `security` 工作流仍每周或手动执行全量扫描，发现漏洞或扫描失败时工作流失败；本地可运行 `mise run security`。

镜像构建、扫描和发布由独立的 `Container` 工作流执行，不属于 `CI gate`。它在 main CI 成功后检查同一次运行的 `CI gate`，并使用该运行的提交 SHA 和 `web-content` 产物。PR、非 main 分支、失败或缺少门禁的 CI 不进入构建。

main push 对应的已合并 PR 作者均为 `renovate[bot]` 时，自动跳过镜像构建和发布；判断依据是 GitHub API 返回的关联 PR 作者和合并 SHA，不是标题、标签或合并操作者。普通 main 更新在构建和扫描通过后发布 SHA 标签，提交仍是 main 最新 SHA 时才发布 `latest`。手动 main CI 成功后仅构建和扫描，不发布。

也可在 main 上手动触发 `Container`，提供本仓库成功的 main `CI` 运行编号 `ci_run_id`；`publish` 默认为 `false`，设为 `true` 才发布。这是显式构建入口，可用于重新构建 Renovate 更新后的 main。其他仓库后续可在满足自身条件后，通过 GitHub API 调用同一个 workflow dispatch 接口，但必须提供有权触发本仓库 Actions 的凭据；调用方的 `GITHUB_TOKEN` 不自动获得跨仓库权限。本仓库仍验证所提供的 CI 运行和门禁，不接受任意源码 ref 或外部产物。外部内容更新后，应先重新运行本仓库 CI，使用新生成并通过检查的内容产物，而不是复用更新前的 CI 运行。Web 产物保留一天，过期后也须重新运行 CI。

例如，先查询 main 的成功 CI，将示例运行编号替换为实际编号，再按需触发构建：

```bash
gh run list --repo heyblog/web --workflow ci.yaml --branch main --status success
gh workflow run container.yaml --repo heyblog/web --ref main -f ci_run_id=123456789 -f publish=false
```

工作流不配置发布并发队列。发布 `latest` 前的 SHA 检查不是原子操作，仍存在检查后 main 更新或多个发布并行执行的竞争窗口。

## Renovate 依赖更新

常规 minor、patch、digest 和 pinDigest 更新集中为 `chore(deps): update non-major dependencies` 批次。每周锁文件维护保持独立 PR。配置当前处于分阶段启用的第一阶段，两类 PR 的自动合并均关闭；完成下述仓库保护设置后才能开启检查通过后的自动 squash 合并。更新窗口仍为上海时间每周一 00:00–05:59；npm 新版本等待七天。

major 更新仍需依赖控制台审批并人工合并；安全漏洞专用 PR 和普通版本 pin 也保持人工合并。自动合并分支落后于 main 时会刷新并重跑检查；其他 Renovate PR 仅在冲突时自动刷新，人工合并前如分支落后需先更新。批次中任一依赖导致检查失败会阻塞整批，不能通过降低门禁合并。

本仓库不通过标签、标题或分支名为人工或其他机器人的 PR 启用自动合并。GitHub 的仓库级开关只是允许使用该功能；具有相应权限的人仍可手动开启其他 PR 的原生自动合并。

启用时先将批次规则与 `lockFileMaintenance` 的 `automerge` 均设为 `false`，上线分组与 CI 门禁。确认新检查成功后，保护 main，要求分支最新且 `CI gate`、`Analyze go`、`Analyze javascript-typescript`、`dependency-review` 全部通过，检查来源限定为 GitHub Actions，不给 Renovate 绕过权限。然后开启仓库自动合并和合并后删分支，最后将上述两处 `automerge` 恢复为 `true`。仓库设置须单独查验，不能仅凭配置文件判断已启用。

合并后删分支适用于所有已合并 PR。main push 的验证不跳过，Renovate PR 合并到 main 后不自动构建镜像。不使用取消旧工作流来降噪；常规批次和每周锁文件维护仍可能分别触发 main CI，内容变化和分支刷新也需要重新验证。

Renovate 仍由 Mend 托管 App 运行，保留现有调度策略。每周窗口限制常规更新的创建时间，不代表 App 只在该窗口运行；窗口外仍可能刷新已有分支，安全漏洞更新也保留其独立策略。不新增自托管 Renovate 工作流。

修改更新策略或门禁后执行 Renovate 官方配置验证器和 `mise run verify`。
