---
name: orchestrate-realtime-backend
description: "在 API/Worker 后端实现数据变更到事件发布与推送的链路。Use when: 需要设计 topic/payload、缓存失效与 SSE 发布。"
---

# Orchestrate Realtime Backend

## 前置条件

- 任务涉及写库后通知前端

## 行为

1. 明确区块级 topic 与最小 payload
2. 确定写库后缓存失效点
3. 发布事件并通过 SSE/等价通道推送
4. 记录并测试事件顺序

## 限制

- 不直接广播整页 HTML 或原始数据库行
- 不依赖进程内事件作为跨进程长期方案
- 必须遵守事件顺序契约

## 联动

- `implement-api-endpoint`
- `implement-api-data-access`
- `run-worker-jobs`
- `implement-realtime-frontend`
- `govern-engineering-rules`
