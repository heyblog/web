# 贡献指南

欢迎参与 zhblogs/core。

本文档旨在帮助贡献者快速完成以下事情：

- 搭建本地开发环境
- 了解 Monorepo 的目录边界
- 按项目规范提交代码与发起 PR

## 适用范围

本仓库聚焦 Web 平台相关代码，采用模块化单体架构设计，并使用 monorepo 管理。当前包含以下核心模块：

如果您要参与其他官方衍生项目，请前往对应仓库查看其独立文档。

## 环境要求

- Node.js >= 24.7.0
- pnpm >= 10.0.0

建议先确认版本：

```bash
node -v
pnpm -v
```

## 快速开始

1. 安装依赖

```bash
pnpm install
```

2. 配置环境变量

```bash
cp .env.example .env.dev
cp .env.example .env.test
cp .env.example .env.prod
```

如果您只做本地开发，至少保证 `.env.dev` 中的核心配置可用（如 `DATABASE_URL`）。

3. 启动开发环境（API + Web）

```bash
pnpm run dev
```

常用拆分命令：

```bash
pnpm run dev:api
pnpm run dev:web
```

## 常用命令

以下命令默认在仓库根目录执行：

```bash
# 代码质量
pnpm run format
pnpm run lint
pnpm run typecheck

# 测试
pnpm run test
pnpm run test:api
pnpm run test:web

# 构建
pnpm run build
pnpm run build:api
pnpm run build:web

# 数据库（Drizzle）
pnpm run db:generate
pnpm run db:migrate
pnpm run db:studio
```

## 项目结构

下面是核心目录与职责说明：

```text
/
├── apps/
│   ├── api/         # API 服务（Fastify）
│   ├── web/         # 前端应用（Astro + Svelte）
│   ├── cloudflare/  # Cloudflare 相关能力目录
│   └── status/      # 状态监控相关目录
├── packages/
│   ├── db/          # Drizzle ORM schema 与数据库工具
│   ├── configs/     # 共享配置（ESLint/Prettier/TS 等）
│   └── utils/       # 通用工具库
└── scripts/         # 仓库级脚本（hooks 等）
```

说明：

- 当前根脚本默认联动开发链路为 `apps/api` 与 `apps/web`。
- 修改共享能力时，请评估是否应放在 `packages/*`，避免在应用中重复实现。

## 贡献流程

推荐流程如下：

1. Fork 仓库并创建分支

建议分支命名：

- `feat/<topic>`
- `fix/<topic>`
- `refactor/<topic>`

2. 开发并保持小步提交

每次提交聚焦单一目标，便于 Review 与回滚。

3. 提交前执行检查

```bash
pnpm run format
pnpm run lint
pnpm run typecheck
pnpm run test
```

4. 发起 Pull Request

PR 描述建议包含：

- 变更背景与目标
- 主要改动点
- 验证方式（命令或截图）
- 潜在影响范围（API、页面、数据结构等）

## Commit 规范

仓库已配置 `commitlint + commit-msg hook`，提交信息需遵循 Conventional Commits。

格式：

```text
<type>(<scope>): <summary>
<type>: <summary>
```

规则：

- `type` 仅允许：`feat`、`fix`、`refactor`、`docs`、`style`、`test`、`build`、`ci`、`chore`、`perf`、`revert`
- `scope` 可选，可写单个或多个模块，例如 `api`、`web`、`api,web`
- `summary` 使用简短摘要，避免尾部句号
- 首行长度需控制在 `72` 个字符以内

示例：

```text
feat(api): add health route
refactor(web): simplify footer layout
chore: update pnpm workspace configuration
```

如果本地 hook 未生效：

```bash
pnpm run hooks:install
```

如果要检查最近一笔提交是否符合规范：

```bash
pnpm run commit:lint
```

## Issue 与功能建议

欢迎通过 Issue 报告问题或提出需求。为提高处理效率，请尽量提供：

- 问题现象与预期行为
- 复现步骤
- 相关日志、截图或报错信息
- 环境信息（Node、pnpm、操作系统）

## 行为准则

请保持友好、尊重、建设性的沟通方式。

我们欢迎不同背景的贡献者，共同提升项目质量。


因此，我们对集博栈的 Robot 进行了规范化管理，针对不同用途的 Robot 基于 [Robots Exclusion Standard](https://www.robotstxt.org/robotstxt.html) 制定了集博栈的 Robot 守则，内容大致如下：

集博栈的 User-Agent 为 `Mozilla/5.0 (compatible; zhblogs-<module>/<version>; +<doc_url>)`，`<module>` 为该 Robot 的模块（用途）名称，`<version>` 为版本号，`<doc_url>` 为对应的 Robot 的详细文档地址。

您可以在您网站的根目录下添加 `robots.txt` 文件，来控制集博栈的 Robot 的访问权限，但是请注意，如果您禁止了集博栈的 Robot 访问您的网站，集博栈将无法收录您的博客数据，并可能导致您的网站数据展示有误。
