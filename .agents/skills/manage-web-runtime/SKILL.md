---
name: manage-web-runtime
description: "在 apps/web 处理中 Astro 渲染策略、数据接入、Actions、同源局部接口、缓存失效和内容来源。"
---

# Manage Web Runtime

## 前置条件

- 任务发生在 `apps/web` 运行时链路

## 行为

1. 先确定路由采用 SSG/SSR/prerender 策略
2. 确定同源接口与写操作入口（Actions/route）
3. 确定缓存与失效策略
4. 若有实时更新，定义区块级重拉路径

## 限制

- 不默认全站 SSR
- 不让浏览器直连内部实现接口
- 列表交互不退化为整页 reload

## 联动

- `build-ui-system`
- `implement-realtime-frontend`
- `orchestrate-realtime-backend`
- `govern-engineering-rules`
