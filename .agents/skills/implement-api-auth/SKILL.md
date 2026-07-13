---
name: implement-api-auth
description: "在 apps/api 实现或调整鉴权、安全与限流能力。Use when: 需要新增 JWT、角色校验、管理接口权限、速率限制、安全插件或认证环境变量。"
---

# Implement API Auth

## 前置条件

- 任务发生在 `apps/api`
- 目标接口需要定义公开/登录/管理/内部边界

## 行为

1. 先定接口分级
2. 再定认证方式与角色模型
3. 再定限流粒度
4. 最后落地插件注册、路由保护与测试

## 限制

- 不把“暂不鉴权”作为默认值
- 不跳过限流
- API 专属变量保持 `API_` 前缀
- 业务通信主键遵循 `id-contract`

## 联动

- `implement-api-endpoint`
- `govern-engineering-rules`
