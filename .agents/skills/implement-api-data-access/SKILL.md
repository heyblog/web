---
name: implement-api-data-access
description: "在 apps/api 使用 Fastify 已挂载的数据库与缓存依赖执行读写。Use when: 需要 app.db.read/app.db.write/app.db.cache 查询或缓存操作。"
---

# Implement API Data Access

## 前置条件

- 任务发生在 `apps/api`
- 需要访问 PostgreSQL 或缓存

## 行为

1. 读取走 `app.db.read`
2. 写入走 `app.db.write`
3. 缓存走 `app.db.cache`
4. schema 与 zod 统一复用 `@zhblogs/db`

## 限制

- 不自建数据库或缓存连接
- 不在 API 层重复定义 schema/enum
- 业务主键统一为 `id`

## 联动

- `define-db-schema`
- `govern-engineering-rules`
