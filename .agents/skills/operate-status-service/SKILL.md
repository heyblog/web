---
name: operate-status-service
description: "在 apps/status 实现状态页、管理员后台、GitHub OAuth、监控探测、邮件通知与 DNS 切换。"
---

# Operate Status Service

## 前置条件

- 任务发生在 `apps/status`

## 行为

1. 先判定变更属于页面、鉴权、监控、DNS、通知或配置
2. 在对应模块完成实现并保持职责边界
3. 补齐核心路径测试

## 限制

- 不泄露管理后台信息到公共页
- 不绕过监控阈值与状态持久化
- 环境变量保持 `STATUS_` 前缀

## 联动

- `build-ui-system`（仅改 UI）
- `govern-engineering-rules`
