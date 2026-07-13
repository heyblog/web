---
name: implement-cloudflare-site-check
description: "在 apps/cloudflare 实现或调整站点探测、Bearer 鉴权、重试分类与返回结构。Use when: 需要修改 /health 或 /check 相关逻辑。"
---

# Implement Cloudflare Site Check

## 前置条件

- 任务发生在 `apps/cloudflare`

## 行为

1. 校验 Bearer token
2. 执行探测与超时重试
3. 标准化错误分类与返回结构
4. 补齐对应测试

## 限制

- 不写数据库
- 不改变既有返回字段语义
- 不绕过鉴权

## 联动

- `run-worker-jobs`（需对齐结果映射时）
- `govern-engineering-rules`
