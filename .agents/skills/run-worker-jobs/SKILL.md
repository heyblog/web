---
name: run-worker-jobs
description: "在 apps/worker 实现 PostgreSQL jobs 队列消费、任务处理、状态回写和健康检查。"
---

# Run Worker Jobs

## 前置条件

- 任务发生在 `apps/worker`

## 行为

1. 按任务类型消费 jobs
2. 执行处理并回写执行状态
3. 维护心跳与健康检查
4. 需要前端同步时发布领域事件

## 限制

- 不另造任务状态体系
- 不跳过状态回写与心跳
- 不在写库后直接请求 web 页面

## 联动

- `define-db-schema`
- `orchestrate-realtime-backend`
- `implement-cloudflare-site-check`
- `govern-engineering-rules`
