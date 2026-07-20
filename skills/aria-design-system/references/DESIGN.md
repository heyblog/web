# Aria Design System 设计文档 / Design Documentation

> **风格 Style**: Flat 2.0 + Minimal · **主色 Primary**: Teal 青瓷 · **圆角 Radius**: ≤ 6px · **模式 Modes**: Light / Dark
>
> 配套文件 Companion files: [`tokens.json`](./tokens.json)（W3C DTCG token 源）· [`preview.html`](./preview.html)（Tailwind v4 组件预览）

---

## 目录 Table of Contents

1. [设计理念 Design Philosophy](#1-设计理念--design-philosophy)
2. [色彩系统 Color](#2-色彩系统--color)
3. [排版 Typography](#3-排版--typography)
4. [间距与布局 Spacing & Layout](#4-间距与布局--spacing--layout)
5. [圆角 Radius](#5-圆角--radius)
6. [深度与层级 Elevation](#6-深度与层级--elevation)
7. [组件规范 Components](#7-组件规范--components)
8. [动效 Motion](#8-动效--motion)
9. [暗黑模式 Dark Mode](#9-暗黑模式--dark-mode)
10. [可访问性 Accessibility](#10-可访问性--accessibility)
11. [Token 治理 Token Governance](#11-token-治理--token-governance)

---

## 1. 设计理念 / Design Philosophy

### 中文

Aria 的目标是**淡雅、优雅、视觉清爽**。它由四条原则协同构成：

| 原则 | 含义 | 落地手段 |
|---|---|---|
| **Flat 2.0** | 平面为主，保留极轻的深度暗示 | 1px 边框划分结构；阴影极淡（静止面 y≤4px、透明度≤6%）；无渐变堆砌、无拟物 |
| **Minimal** | 少即是多 | 大量留白；每屏一个视觉焦点；装饰元素趋近于零 |
| **颜色优先** | 色彩承担「重要性」信号 | 实心青瓷只给页面最高优先级动作；次级强调用 tint（teal-50 底 + teal-700 字）；状态一律语义色 |
| **结构优先** | 结构承担「组织」信号 | 4px 间距节奏、1px 分隔线、明确栅格来组织信息，不靠色块划分区域 |

两种「优先」并不冲突：**结构负责让页面读得清，颜色负责让页面读得快**。当二者竞争时，先用结构（留白/边框）解决，色彩只在结构不足以表达优先级时介入。

「淡雅清爽」的具体来源：主色是**青瓷（Celadon Teal，600 阶 `#0F7B80`）**——瓷器釉色般清冽的青绿，天然内敛，联想到水与数据的流动，贴合「博客聚合、RSS 数据流、可视化」的产品语境，清雅而不喊叫；中性色 light 模式使用带蓝调的冷灰（slate），画布是近白的 slate-50；dark 模式为避免蓝色偏移改用**中性深灰（zinc）**，画布为 zinc-950 而非纯黑；青瓷在页面中的覆盖面积被刻意压低（经验值 <10%），从而在出现时获得最大注意力。

### English

Aria aims to feel **calm, elegant, and visually clean**. Four principles work together: **Flat 2.0** keeps surfaces flat but allows whisper-quiet depth cues (1px borders, shadows ≤6% opacity); **Minimal** means generous whitespace and one focal point per screen; **color-first** reserves the solid primary for the single highest-priority action, using tinted fills for secondary emphasis; **structure-first** organizes information through a 4px spacing rhythm and 1px dividers rather than colored blocks. Structure makes the page *legible*; color makes it *scannable*. The primary is **celadon teal** — a cool, porcelain-like blue-green that evokes flowing water and flowing data, fitting a product about aggregating blogs, RSS streams, and visualization. Light-mode neutrals are cool slate grays; dark mode switches to neutral zinc grays to avoid any blue cast. Primary coverage is deliberately kept below ~10% of any screen so that it commands attention when it appears.

---

## 2. 色彩系统 / Color

### 2.1 主色阶 Primary Scale — 青瓷 Teal

> 自定义色阶：青瓷釉色般的蓝绿——清冽、内敛，与语义色 success（emerald，偏绿）和 info（sky，偏蓝）保持可辨识的色相距离。

| Step | Hex | Light 模式用途 | Dark 模式用途 |
|---|---|---|---|
| 50 | `#EFF8F8` | tint 底色（次级按钮、选中态） | — |
| 100 | `#D9EEEF` | tint hover | 文本选中前景 |
| 200 | `#B3DEE0` | 文本选中背景 | — |
| 300 | `#7FC5C9` | disabled 主按钮 | dark tint 前景 |
| 400 | `#3FA6AC` | — | focus ring |
| 500 | `#158187` | — | **主交互色**（白字 ≈4.6:1） |
| 600 | `#0F7B80` | **主交互色** / focus ring（白字 ≈5.0:1） | hover |
| 700 | `#0C666B` | hover / tint 前景 | active |
| 800 | `#0B5357` | active | 文本选中背景 |
| 900 | `#0A4346` | — | dark tint 底（40% 透明）/ disabled |
| 950 | `#062D2F` | 深强调 / 文本选中前景 | — |

Dark 模式 focus ring 使用 teal-400，在中性深灰上清晰可见。注意 teal 中间调承载白字的亮度上限较低，故 dark 主交互色从 teal-500 开始，并在 hover/active 时逐步加深，以持续满足白字对比度。

### 2.2 中性色阶 Neutral Scale — Slate (Light) / Zinc (Dark)

- **Light 模式**：Slate 50–950 全阶——带蓝调的冷灰，在高亮度下呈现干净的清爽感。
- **Dark 模式**：Zinc（100/400/500/700/800/900/950）——纯中性灰，零蓝色偏移；低亮度下 slate 的蓝色相会浮现，故 dark 不用 slate。画布使用 zinc-950，抬升面使用 zinc-900，次级表面使用 zinc-800，全部取自标准色阶。

### 2.3 语义色 Status Colors

| 语义 | 色相 | Light solid | Dark fg |
|---|---|---|---|
| Success | Emerald | `emerald-600 #059669` | `emerald-400 #34D399` |
| Warning | Amber | `amber-500 #F59E0B` | `amber-400 #FBBF24` |
| Danger | Rose | `rose-600 #E11D48` | `rose-400 #FB7185` |
| Info | Sky | `sky-600 #0284C7` | `sky-400 #38BDF8` |

每个语义色提供四个角色：`fg`（文字/图标）、`bg`（tint 底）、`border`（色条/描边）、`solid`（实心填充）。
`fg` + `bg` 是默认文字状态配对；`solid` 用于高饱和图形或控件填充，不隐含白色前景，带文字的实心状态控件必须单独满足 AA 对比度。

### 2.4 语义映射 Semantic Mapping（`semantic.color.{light|dark}`）

| Token | Light | Dark | 用途 |
|---|---|---|---|
| `bg.canvas` | slate-50 | zinc-950 | 页面背景 Page background |
| `bg.surface` | white | zinc-900 | 卡片/面板/输入框 Cards, panels, inputs |
| `bg.subtle` | slate-100 | zinc-800 | hover 填充、斑马纹 Hover fills, stripes |
| `fg.default` | slate-900 | zinc-100 | 主文字 Primary text |
| `fg.muted` | slate-500 | zinc-400 | 次要文字 Secondary text |
| `fg.subtle` | slate-400 | zinc-500 | 占位符/禁用 Placeholders, disabled |
| `border.default` | slate-200 | zinc-800 | 结构分隔线 Structural dividers |
| `border.strong` | slate-500 | zinc-500 | 输入框/控件边界（相邻表面 ≥3:1）Input and control boundaries |
| `border.focus` | teal-600 | teal-400 | 焦点环 Focus ring |
| `action.primary.default` | teal-600 | teal-500 | 主动作 Primary action；dark hover/active 依次使用 teal-600/700，持续保持白字 AA 对比度 |
| `action.primary.tintBg` | teal-50 | teal-900 @40% | 次级强调底 Tinted emphasis |
| `action.primary.tintFg` | teal-700 | teal-300 | 次级动作与文字链接前景 Secondary action and link foreground |

### 2.5 对比度要求 Contrast Requirements

- 正文 Body text: **≥ 4.5:1**（WCAG AA）
- 大字（≥18px semibold）与图形 Large text & graphics: **≥ 3:1**
- 已验证组合 Verified pairs: `slate-900/slate-50` ≈ 17:1；`slate-500/white` ≈ 4.8:1；`slate-500/slate-50` ≈ 4.6:1；`white/#0F7B80`（light 主按钮）≈ 5.0:1；`white/#158187`（dark 主按钮）≈ 4.6:1；`zinc-100/zinc-950` ≈ 18:1；`zinc-400/zinc-900` ≈ 6.9:1；`zinc-500/zinc-900` ≈ 3.7:1（dark 控件边界）。
- `fg.subtle` 不达正文 AA，**仅**允许用于占位符、禁用态和无文字含义的装饰图标；Caption、表头和辅助说明必须使用 `fg.muted`。 `fg.subtle` is reserved for placeholders, disabled states, and decorative icons; readable captions use `fg.muted`.

---

## 3. 排版 / Typography

### 字体栈 Font Stack

| 角色 | 字体序列 | 用途 |
|---|---|---|
| Sans | Inter → Noto Sans SC → system UI → sans-serif | UI 与正文 |
| Mono | JetBrains Mono → system monospace → monospace | 代码与等宽数据 |

Inter 覆盖拉丁字符，Noto Sans SC 无缝回退中文；两者 x-height 相近，混排不跳行。

### 字号阶梯 Type Scale

| Token | 大小 | 行高 | 字重 | 场景 Usage |
|---|---|---|---|---|
| `4xl` | 36px | 1.2 tight | 700 | 页面主标题 Display |
| `3xl` | 30px | 1.2 tight | 700 | 区块大标题 |
| `2xl` | 24px | 1.2 tight | 600 | Heading |
| `xl` | 20px | 1.4 snug | 600 | 卡片标题 Card title |
| `lg` | 18px | 1.4 snug | 500 | 副标题 Subtitle |
| `base` | 16px | 1.5 normal | 400 | 正文 Body |
| `sm` | 14px | 1.5 normal | 400/500 | UI 默认、次要文字 UI default |
| `xs` | 12px | 1.5 normal | 400/500 | Caption、徽标 Badge |

**字重规则 Weight rules**: 400 正文；500 交互元素（按钮/标签/导航）；600 标题；700 仅 Display。全局不使用 300 及以下——细字重在暗底上发虚，与「清爽」相悖。

---

## 4. 间距与布局 / Spacing & Layout

**4px 基准 4px base**: `4 / 8 / 12 / 16 / 24 / 32 / 48 / 64`

三级间距节奏 Three-tier rhythm：

| 层级 | 取值 | 例 |
|---|---|---|
| 组件内 Within component | 4–12px | 图标与文字 gap 6px；输入框内边距 12px |
| 组件间 Between components | 16–24px | 表单字段间 20px；卡片网格 gap 16–24px |
| 区块间 Between sections | 48–64px | 页面 section 间 64px |

布局容器 max-width **1280px**；卡片默认内边距 **24px**（`space.6`）。留白宁多勿少——Minimal 的层级感首先来自间距差，而不是分隔线数量。

### 4.1 响应式断点 / Responsive Breakpoints

所有响应式规则采用 mobile-first 的 `min-width` 查询；低于 `xs` 使用基础样式，不新增隐式设备断点。

| Token | rem | px | 主要布局阶段 |
|---|---:|---:|---|
| `breakpoint.xs` / `--breakpoint-xs` | 30rem | 480px | 大屏手机、紧凑双列入口 |
| `breakpoint.sm` / `--breakpoint-sm` | 48rem | 768px | 平板、控件切换为紧凑高度 |
| `breakpoint.md` / `--breakpoint-md` | 64rem | 1024px | 小型笔记本、主内容多列 |
| `breakpoint.lg` / `--breakpoint-lg` | 80rem | 1280px | 标准桌面、显示侧边导航 |
| `breakpoint.xl` / `--breakpoint-xl` | 90rem | 1440px | 宽桌面、扩大内容留白 |
| `breakpoint.2xl` / `--breakpoint-2xl` | 120rem | 1920px | 超宽屏、保持内容宽度上限 |

组件不得按设备型号分支；只依据内容拥挤程度选择上述断点。页面内容宽度仍受 1280px 容器约束，`xl` 与 `2xl` 主要增加外侧留白，避免行长失控。

---

## 5. 圆角 / Radius

| Token | 值 | 用途 |
|---|---|---|
| `radius.xs` | 2px | 微元素：tooltip、关闭按钮热区 |
| `radius.sm` | 4px | checkbox、badge、菜单项、色块 |
| `radius.md` | **6px（硬上限 hard cap）** | 按钮、输入框、卡片、modal、dropdown |
| `radius.full` | 9999px | **仅限例外清单** |

**`full` 例外清单 Exception list**（超出此清单使用 full 视为违规）：

1. Avatar 头像
2. 胶囊徽标 Pill badge / 状态点 status dot
3. Switch 开关（轨道与滑块）
4. Radio 单选控件
5. Progress 进度条轨道
6. Stepper 步骤圆点 step markers

其余一切界面元素圆角 ≤ 6px。小圆角保持利落的几何感，是这套系统「优雅克制」气质的来源之一。

---

## 6. 深度与层级 / Elevation

**边框优先，阴影仅用于浮层。** Borders first; shadows are reserved for floating layers.

| 层级 | z-index | 深度手段 | Shadow token |
|---|---|---|---|
| 0 画布 Canvas | — | `bg.canvas` | — |
| 1 静止面 Resting surface（卡片/表格） | — | 1px `border.default`；阴影可省略 | `shadow.2xs` |
| 2 悬浮态 Hover（仅可点卡片） | — | translateY(-1px) + | `shadow.xs` |
| 3 吸附层 Sticky（header） | 30 | 1px 底边框 + backdrop-blur | — |
| 4 浮层 Popover（dropdown/tooltip） | 20–40 | 1px 边框 + | `shadow.sm` |
| 5 遮罩层 Overlay（modal/toast） | 50 | scrim（light `#0F172A` @60% / dark `#09090B` @70%）+ | `shadow.md` |

阴影直接采用 Tailwind CSS v4 的标准层级，并通过降低使用等级保持 Flat 2.0 的克制：

```
2xs: 0 1px 0 rgb(0 0 0 / 0.05)
xs:  0 1px 2px 0 rgb(0 0 0 / 0.05)
sm:  0 1px 3px 0 rgb(0 0 0 / 0.10), 0 1px 2px -1px rgb(0 0 0 / 0.10)
md:  0 4px 6px -1px rgb(0 0 0 / 0.10), 0 2px 4px -2px rgb(0 0 0 / 0.10)
```

Dark 模式下阴影几乎不可见，层级改由**更亮的表面色 + 1px 边框**表达（见 §9）。

---

## 7. 组件规范 / Components

> 所有值引用 token 名，视觉与交互状态由 `preview.html` 展示。 All values reference token names; `preview.html` demonstrates their visual and interaction states.

### 7.1 Button 按钮

**解剖 Anatomy**: 容器（`button.radius` = radius.md）+ 可选前/后置图标（16px，gap 6px）+ 标签（fontWeight.medium）。

**尺寸 Sizes**: sm 32px / md 36px（默认）/ lg 40px；paddingX 12 / 16 / 20px。

**变体与状态矩阵 Variant × State matrix**（light 模式 token）：

| 变体 | Default | Hover | Active | Disabled |
|---|---|---|---|---|
| Primary | `action.primary.default` 底 + 白字 | `action.primary.hover` | `action.primary.active` + scale 0.96 | `action.primary.disabled` |
| Secondary | `tintBg` 底 + `tintFg` 字 | teal-100 | scale 0.96 | opacity 50% |
| Outline | `bg.surface` + 1px `border.strong` | `bg.subtle` | scale 0.96 | opacity 50% |
| Ghost | 透明 + `fg.muted` | `bg.subtle` + `fg.default` | scale 0.96 | opacity 50% |
| Danger | `status.danger.solid` 底 + 白字 | opacity 90% | scale 0.96 | opacity 50% |
| Link | `action.primary.tintFg` 文字 | underline (offset 4px) | — | opacity 50% |

**规则 Rules**: 每屏至多一个 Primary；Loading 态使用 Tailwind 标准 spinner（1s linear rotate），声明 busy 状态并禁用点击；Icon button 为 36×36 正方形。仅图标按钮必须提供可访问名称。

### 7.2 Input / Form 表单

| 状态 | 边框 | 其他 |
|---|---|---|
| Default | 1px `border.strong` | placeholder 用 `fg.subtle` |
| Focus | `border.focus` + 2px focus outline（offset 2px） | 边框色过渡 duration.fast；键盘焦点轮廓即时出现 |
| Error | `status.danger.solid` | 下方 `help-error` 提示 + shake 动画（280ms） |
| Disabled | `border.default` | `bg.subtle` 底、`fg.subtle` 字、not-allowed 光标 |

移动端高度 44px，`sm` 起为 40px；paddingX 12px、radius.md。Label 在上（sm/medium，间距 6px），帮助文字在下（xs/`fg.muted`）。原生 Select 使用 16px 自定义箭头，light/dark 分别采用对应的 muted 前景；hover 只增强边框，disabled 保留箭头但降低对比度。

**Checkbox** 16px / radius.sm，选中填充 `action.primary`，白色对勾由 0→12px 显现；indeterminate 使用白色短横线并保留混合状态语义。**Radio** 16px 圆形，选中圆点由 scale(0)→1。**Switch** 36×20px、radius.full（例外清单），thumb 16px，滑动 duration.base。三者使用可中断 transition，按下 scale(0.96)，视觉尺寸不变；整行 Label 在触控布局为 44px、`sm` 起为 40px。**Slider** 使用当前语义主色强调，轨道与 thumb 保持浏览器原生交互，并采用相同命中高度。

### 7.3 Card 卡片

`card.padding` = 24px，radius.md，1px `border.default`，`shadow.2xs`。仅在整张卡片确实可操作且设备具有精细指针时使用 hover：translateY(-1px) + `shadow.xs` + `border.strong`；位移与边框使用 duration.base，阴影层级即时切换而不插值，并提供键盘等价操作。纯数据卡片保持静止且不显示 pointer 光标。卡片内不再嵌套阴影。

### 7.4 Badge / Tag

高 20px、paddingX 8px、fontSize.xs/medium。方角变体 radius.sm；胶囊变体 radius.full（例外清单）。语义变体用 `status.*.bg` + `status.*.fg` tint 组合，**不用实心底**——保持页面轻盈。Tag 可关闭：× 图标 12px，透明命中区在触控布局扩展至 44×44px、`sm` 起为 40×40px，点击后 duration.base 淡出移除。

### 7.5 Table 表格

行高 48px、单元格 paddingX 16px。表头：xs/medium/`fg.muted`，底部 1px 分隔。斑马纹用 `bg.subtle` @50%；行 hover `bg.subtle`，duration.fast。状态列用胶囊 badge + 状态点。窄屏允许表格区域横向滚动，不压缩关键列。

### 7.6 Tabs 标签页

高 40px，底部 1px `border.default` 轨道。激活态：`fg.default` + medium + 2px 主色下划线指示器；等宽标签使用 `transform: translateX()` 滑动指示器（duration.base / easing.standard），避免动画化布局属性。指针点击保留滑动反馈；方向键及 Home/End 导航即时定位，不播放位移动画。

### 7.7 Dropdown 下拉菜单

minWidth 192px、radius.md、1px 边框 + `shadow.sm`、内边距 4px。菜单项在触控布局高 44px、`sm` 起 40px，radius.sm；hover/focus-visible 使用 `bg.subtle`，危险项使用 `status.danger.fg` + `status.danger.bg`。入场：`scale(0.97) translateY(-4px)` → 1，origin 随触发器锚点变化，duration.base / easing.out；关闭以 `scale(0.99) translateY(-2px)` + fade 使用 duration.fast。点击外部或 Esc 关闭；打开后方向键、Home/End 在菜单项间移动，关闭时焦点回到触发器。

### 7.8 Modal 对话框

maxWidth 480px、radius.md、`shadow.md`、内边距 24px。遮罩 `overlay.scrim`（light：slate-900 @60%；dark：zinc-950 @70%）。入场：遮罩 fade，面板 `scale(0.96)`→1 + fade（duration.base / easing.out）；关闭使用 duration.fast。关闭方式：Esc / 点遮罩 / 关闭按钮；打开时锁定 body 滚动，将焦点移入并圈定在面板内，关闭动画完成后隐藏并归还焦点。

### 7.9 Toast 轻提示

宽 360px、radius.md、`shadow.md`，固定右下角，普通消息使用礼貌播报，错误消息使用即时播报。入场 translateX(16px)→0 + fade（duration.slow / easing.out）；4s 自动消退（出场 duration.base）；状态使用可中断 transition，快速关闭时从当前视觉位置继续；hover 或键盘焦点进入时暂停计时，可手动关闭；多条纵向堆叠 gap 8px。

### 7.10 Alert / Banner

Alert：radius.md、`status.*.bg` tint 底 + 左侧 3px `status.*.border` 色条 + 语义图标（18px）。四个语义变体。Banner：`tintBg` 底、无色条、可关闭（duration.base 淡出）。

### 7.11 Tooltip

radius.sm、`bg.inverse` 底 + `fg.inverse` 字、fontSize.xs、padding 4×8px。精细指针 hover 或键盘 focus 延迟 **80ms** 显示，fade + 2px 上移（duration.fast）；离开不延迟。触发元素通过可访问描述关联提示内容。纯 CSS 实现。

### 7.12 其他 Others

- **Avatar**: 28/36/48px，radius.full（例外清单）；堆叠组 -8px 负间距 + 2px surface 描边。
- **Progress**: 轨道 6px 高、radius.full（例外清单）、`bg.subtle` 底 + 主色填充；确定态以左侧为原点使用 scaleX 变化（duration.slow / easing.standard），不确定态使用 1.4s linear 循环滑块。确定态提供当前值，不确定态仅声明加载语义。
- **Skeleton**: radius.sm、`bg.subtle` 底，使用 Tailwind 标准 2s opacity pulse；骨架形状对辅助技术隐藏，由容器播报加载状态。
- **Empty state**: 图标容器 48px/`bg.subtle`，标题 sm/semibold，描述 `fg.muted`，主操作按钮。
- **Stepper**: 已完成 = 主色实心 + 对勾；当前 = 2px 主色描边 + 主色数字；未来 = 1px `border.strong` + `fg.muted`；连接线 1px（完成段主色）。
- **Pagination**: 视觉尺寸可保持 32px，透明命中区在触控布局补至 44px、`sm` 起补至 40px；当前页 `tintBg` + `tintFg`。
- **Breadcrumb**: `fg.muted` 链接 + chevron 分隔，当前页 `fg.default`/medium；可点击层级采用相同的 44px / 40px 响应式命中区。

---

## 8. 动效 / Motion

### 8.1 Token

| Token | 值 | 场景 |
|---|---|---|
| `duration.micro` | 80ms | Tooltip 意图延迟、短序列节拍 |
| `duration.fast` | 150ms | 颜色/边框/透明度 hover |
| `duration.base` | 250ms | Switch、Tabs、Accordion、浮层入场 |
| `duration.medium` | 350ms | Toast 退出、次级面板关闭 |
| `duration.slow` | 400ms | Toast 入场、内容面板入场 |
| `duration.emphasis` | 500ms | 分组内容 reveal |
| `easing.standard` | `cubic-bezier(0.4, 0, 0.2, 1)` | 可逆状态与屏内位移过渡 |
| `easing.out` | `cubic-bezier(0.22, 1, 0.36, 1)` | 元素、浮层与内容进出场 |
| `easing.linear` | `linear` | Spinner 与不确定进度 |
| `easing.pulse` | `cubic-bezier(0.4, 0, 0.6, 1)` | Tailwind Skeleton pulse |

### 8.2 交互动画清单 Interaction Inventory

| 交互 | 动画 | 时长/缓动 |
|---|---|---|
| 按钮 hover | 背景色过渡 | fast / standard |
| 按钮按下 | scale(0.96) | fast / out |
| 输入框聚焦 | 边框色过渡；键盘 focus outline 即时出现 | fast / standard |
| Checkbox / Radio | 填充与选中符号 scale/opacity；按下 scale(0.96) | fast / out |
| 表单错误 | 水平 shake：6px → -6px → 4px → 0 | 280ms / out |
| 可点卡片 hover | 精细指针下 translateY(-1px) + 边框过渡；阴影层级即时切换 | base / standard |
| Switch 切换 | thumb 平移 16px + 轨道变色 | base / standard |
| Tabs 切换 | 指针点击时指示器 transform 滑动；键盘切换即时定位 | base / standard |
| Dropdown 开关 | scale 0.97→1 + fade 入；scale 0.99 + fade 出；chevron 旋转 | base 入 / fast 出 / out |
| Modal 开关 | 遮罩 fade；面板 scale 0.96→1 + fade 入，反向出 | base 入 / fast 出 / out |
| Toast 进出 | 可中断 transition：translateX 16px→0 fade 入；反向出 | slow 入 / medium 出 / out |
| Tooltip | 精细指针 hover / 键盘 focus 延迟 80ms，fade + 2px 上移；离开无延迟 | fast / out |
| 手风琴 | grid-template-rows 0fr→1fr + chevron 旋转 | base / standard |
| 主题切换 | sun/moon fade + rotate；reduced motion 仅保留 fade | base / out |
| 滚动入场 | 标题、说明、内容块分别 fade + translateY 12px→0；每项 stagger 40ms，总延迟 <300ms | emphasis / out |
| Skeleton | opacity pulse | 2s / pulse 循环 |
| Spinner | 360° 旋转 | 1s linear 循环 |

### 8.3 原则与降级 Principles & Reduced Motion

1. 动效**传达因果**，不表演——一切入场方向与触发源一致（toast 从右缘来即向右缘去）。
2. 位移幅度 ≤ 16px、缩放幅度 ≥ 0.96——克制的幅度维持「淡雅」。
3. 交互状态使用可中断 transition；只声明实际变化的属性，不使用 `transition: all`。位移与缩放优先使用 `transform`/`scale` 与 `opacity`；阴影不做插值；手风琴动态高度是必要例外。
4. 页面入场拆分为语义内容块，stagger 单步 40ms 且总延迟小于 300ms；退出比入场更快、更轻。
5. `will-change` 只允许用于 `transform`、`opacity`、`filter`，且仅在浮层首帧确有抖动时使用。
6. **Reduced motion 降级**：用户选择减少动态效果时，取消位移、缩放、动态高度和循环动画；Skeleton、Spinner 与不确定进度保留静态可见状态；浮层只保留不超过 duration.fast 的透明度反馈；滚动入场直接显示终态，平滑滚动关闭；功能、状态和焦点顺序保持不变。

---

## 9. 暗黑模式 / Dark Mode

### 原理 Mechanism

语义层提供 light / dark 两套同构角色，组件仅消费当前模式的语义角色，因此换肤不改变组件结构。预览默认跟随系统偏好，并记住用户主动选择，避免主题闪烁。
主题切换时语义颜色即时重映射，避免全页颜色同时插值造成不一致过渡和额外重绘；只有 sun/moon 状态图标使用 duration.base 动效。

### Dark 调整规则 Adaptation Rules

| 规则 | 说明 |
|---|---|
| **中性色换族** | dark 不沿用 slate 而改用 zinc——slate 的蓝色相在低亮度下会浮现为「背景发蓝」，zinc 是零蓝偏移的纯中性灰 |
| **保证交互对比** | 主色 teal-600 → teal-500；hover/active 使用 teal-600/700，使白色标签在所有交互状态保持 AA |
| **降饱和 tint** | tint 底从 teal-50 改为 teal-900 @40% 透明，避免暗底上大面积色块 |
| **语义色亮化** | 状态前景统一升两档（600 → 400），底色用 950 深色调 @30% 透明 |
| **画布不用纯黑** | zinc-950 是中性深灰，避免纯黑造成的生硬对比 |
| **层级靠亮度** | 阴影在暗底不可见 → 越高的层用越亮的表面（canvas zinc-950 → surface zinc-900 → subtle zinc-800）+ 1px 边框 |
| **边框整体下调** | border.default slate-200 → zinc-800，保持「隐约可见」的相同感知强度 |

---

## 10. 可访问性 / Accessibility

- **焦点 Focus**: 全局 `:focus-visible` 2px `border.focus` 外环 + 2px offset；仅键盘触发，鼠标点击不显示。
- **键盘 Keyboard**: 所有交互组件可 Tab 达；Modal 打开时焦点移入并圈定、Esc 关闭、关闭后焦点归还触发元素；Dropdown 支持 Esc、方向键和 Home/End；Tabs 支持方向键与 Home/End，并即时切换指示器；Accordion 同步展开状态。
- **ARIA**: Modal `role="dialog" aria-modal="true" aria-labelledby aria-describedby`；Toast 按消息优先级使用 status/alert；Tooltip `role="tooltip"` 并由触发元素 `aria-describedby` 关联；Tabs、Dropdown、Accordion 使用对应角色与状态；图标按钮必须携带可访问名称；导航地标以标签区分。
- **对比度 Contrast**: 见 §2.5；禁用态是唯一允许低于 3:1 的场景。
- **动效 Motion**: 尊重 `prefers-reduced-motion`（§8.3）。
- **触达面积 Hit area**: 触控布局最小可点区域 44×44px，`sm` 起的紧凑桌面布局至少 40×40px；紧凑图标通过透明命中区补足，不改变视觉尺寸，且相邻命中区不得重叠。

---

## 11. Token 治理 / Token Governance

### 11.1 三层架构 Three Layers

```
primitive（原始值：teal.600, space.4, radius.md）
    ↓ 引用 referenced by
semantic（用途别名：action.primary.default, bg.surface — light/dark 双集合）
    ↓ 引用 referenced by
component（组件级：button.radius, modal.shadow）
```

规则：颜色必须经过语义层；组件层只保留几何、排版、层级与动效约束，并声明所需的语义颜色角色。主题切换只改变语义角色映射，不改变组件规范。

### 11.2 Tailwind 可表达性约束 Tailwind Feasibility

- 除青瓷主色阶外，色彩、字号、间距、圆角、阴影与时长均落在 Tailwind CSS v4 的标准尺度内。
- 常规状态变化使用 Tailwind 的颜色、边框、透明度、transform、duration 与 easing 能力；不使用 `transition: all`。
- 仅动态高度、错误 shake、进度不确定态、滚动入场与浮层出入场需要小范围自定义关键帧或状态选择器；均只服务明确反馈，并提供 reduced-motion 降级。
- 预览不依赖复杂动效库；交互所需图标保持单一图标体系与一致描边。

### 11.3 三文件一致性 Three-file Consistency

1. `DESIGN.md` 定义设计意图、组件行为和可访问性边界。
2. `tokens.json` 保存可度量的原始值、语义角色与组件约束。
3. `preview.html` 只负责展示上述规范；任何预览特例不得反向成为未记录的新规则。

变更顺序固定为：先确认设计规则，再更新 token，最后校准预览。三者出现冲突时，以可访问性要求和语义角色定义优先。

### 11.4 使用守则 Do & Don't

| Do | Don't |
|---|---|
| 颜色经语义角色使用 | 组件直接绑定某一模式的原始色值 |
| 每屏一个最高优先级动作 | 多个实心主色按钮竞争注意力 |
| 浮层才使用中高等级阴影 | 静止卡片使用大阴影 |
| radius.full 仅限例外清单六类 | 给按钮或卡片使用胶囊圆角 |

---

*Aria Design System v1.0 · 2026-07 · 三件套：tokens.json / DESIGN.md / preview.html*
