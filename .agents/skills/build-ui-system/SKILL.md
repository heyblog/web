---
name: build-ui-system
description: "在前端 UI 任务中统一页面骨架、设计 token、布局主题与组件实现。Use when: 新增或调整页面、组件、主题、样式、响应式布局、内容页视觉、管理后台 UI 或需要检查设计规范一致性。"
---

# Build UI System

## 使用时机

- `apps/web` 页面、布局、组件、样式、主题、响应式或可访问交互。
- 设计规范、design token、视觉一致性、light/dark 表现检查。
- 内容页、状态页、管理页、表单页接入现有 UI 体系。

## 参考顺序

1. `references/design-spec.md`：Flat 2.0、布局、颜色、排版、圆角、组件语义。
2. `references/design-tokens.md`：可执行 token contract。
3. `../govern-engineering-rules/references/ui-baseline.md`：跨 skill UI 基线。
4. 当前实现：`apps/web/src/styles/tailwind.css`、`apps/web/src/styles/base.css`、相关 layout/component。

参考与代码不一致时，以 token contract 和 UI 基线为准，按现有代码结构做最小一致性修正。

## 执行规则

- 颜色使用 contract token：`--color-bg`、`--color-bg-raised`、`--color-fg*`、`--color-line*`、语义色、状态点色。
- 背景层级使用 `--color-bg` 与 `--color-bg-raised`；轻微分层使用 canonical `color-mix()` arbitrary class。
- 正文基线使用 `16px`；常规 UI 字重使用 `400/500/600`。
- 圆角使用 `rounded-sm`、`rounded-md`、`rounded-full`，对应 `3px/5px/pill`。
- Tailwind scale 覆盖的间距、尺寸、字号、圆角，使用原生 utility。
- CSS 变量 token 使用 Tailwind 4 括号变量语法：`bg-(--color-bg)`、`text-(--color-fg)`、`border-(--color-line)`。
- CSS 函数使用 Tailwind canonical arbitrary class：`bg-[color-mix(in_srgb,var(--color-bg)_82%,transparent)]`、`border-[color-mix(in_srgb,var(--color-line)_72%,transparent)]`。
- Tailwind 色板使用官方 utility：`bg-slate-200`、`text-stone-700`、`border-red-700/20`。
- 页面层级通过细边线、留白、排版和少量 raised surface 表达。
- 状态表达使用文字、细线、状态点。
- 浮层可使用单层阴影；普通页面区块以边线和背景层级表达。
- 嵌套内容使用标题、分割线、密度和间距表达层级。
- light/dark 同步检查文字、边框、hover、focus、disabled、浮层背景。

## 实现流程

1. 定位目标页面或组件的现有模式，沿用本地结构、命名和交互。
2. 按页面类型套用 `references/design-spec.md`：公开页、内容页、管理后台、表单、浮层。
3. 共享问题优先改 token 或共享组件；局部问题局部修正。
4. 内容页优先复用 `MarkdownContentLayout`、`ContentBody`、`ContentToc`。
5. 表单、弹窗、combobox、toast、popover 沿用 shared UI 组件语义和状态。
6. 实时区块联动 `implement-realtime-frontend`，保持局部刷新。
7. Astro 渲染策略、内容集合、同源接口联动 `manage-web-runtime`。

## 验收

- token 来自 contract；新增 token 同步记录。
- Tailwind class 使用当前语法和原生 scale。
- 圆角、字号、间距、背景层级符合参考规范。
- mobile/desktop 无重叠、溢出、布局跳动。
- light/dark 下文字、边框、hover、focus 清晰。
- 运行相关格式、类型或构建检查；无关阻断需记录。

## 联动

- `manage-web-runtime`：Astro 页面、内容集合、路由、渲染策略、同源接口。
- `implement-realtime-frontend`：浏览器实时事件、局部刷新、交互状态保持。
- `orchestrate-realtime-backend`：实时事件 payload、topic、缓存失效链路。
- `govern-engineering-rules`：全局 UI 基线、测试约束、跨 skill 规则收口。
