---
name: implement-api-endpoint
description: "规范在 apps/api 创建或重构 Fastify 接口。Use when: 需要定义输入输出、鉴权、限流、数据访问、缓存、测试和环境变量。"
---

# Implement API Endpoint

## 前置条件

- 任务发生在 `apps/api` 路由层

## 行为

1. 明确接口边界与可见性
2. 定义输入输出校验
3. 定义数据访问与缓存策略
4. 定义鉴权与限流策略
5. 完成路由注册和测试

## 限制

- 不遗漏“路由/校验/访问/鉴权/测试”任一环节
- 接口用于局部刷新时，不返回整页耦合数据
- 业务通信主键统一为 `id`

## 联动

- `implement-api-auth`
- `implement-api-data-access`
- `orchestrate-realtime-backend`（当接口参与实时更新）
- `govern-engineering-rules`
