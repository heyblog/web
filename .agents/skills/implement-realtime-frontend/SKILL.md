---
name: implement-realtime-frontend
description: "在浏览器侧实现 EventSource/实时消费并进行区块级局部刷新。Use when: 需要在 web/status 等页面消费实时事件并保持交互状态。"
---

# Implement Realtime Frontend

## 前置条件

- 页面存在可局部刷新的动态区块

## 行为

1. 初始化实时订阅并处理重连
2. 根据 topic 命中目标区块进行重拉
3. 保留筛选/排序/分页等用户状态
4. 对重复事件做去抖或合并

## 限制

- 不使用整页 reload 替代局部刷新
- 不破坏同源边界
- 不在事件消费中重置用户当前视图状态

## 联动

- `manage-web-runtime`
- `build-ui-system`
- `orchestrate-realtime-backend`
- `govern-engineering-rules`
