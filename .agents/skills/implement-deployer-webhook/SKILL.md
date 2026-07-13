---
name: implement-deployer-webhook
description: "在 apps/deployer 实现或重构 webhook 接收、验签、部署状态流转、脚本调用、健康检查和回滚。"
---

# Implement Deployer Webhook

## 前置条件

- 任务发生在 `apps/deployer`

## 行为

1. 处理 webhook 接收与 HMAC 校验
2. 执行部署脚本并记录状态流转
3. 失败时执行回滚并回写结果

## 限制

- 不跳过验签
- 状态定义与 `packages/db` 保持一致
- 不引入与当前模块不匹配的大框架

## 联动

- `define-db-schema`
- `govern-engineering-rules`
