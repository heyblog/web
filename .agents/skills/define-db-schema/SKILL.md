---
name: define-db-schema
description: "在 packages/db 维护 constants、enums、schema、zod、migration 与导出链。Use when: 需要新增或修改 PostgreSQL 数据结构。"
---

# Define DB Schema

## 前置条件

- 任务发生在 `packages/db`

## 行为

1. 维护 constants -> enums -> schema -> zod -> exports 全链路
2. 确保导出可被上游稳定复用

## 限制

- 不在业务模块重复定义 enum/zod
- 不遗漏导出
- 不保留死字段和语义重复字段
- 生成 migration 时请提示，让用户手动执行操作

## 联动

- `implement-api-data-access`
- `implement-deployer-webhook`
- `run-worker-jobs`
- `govern-engineering-rules`
