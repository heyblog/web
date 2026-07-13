---
name: govern-engineering-rules
description: "跨 skills 的全局规则收口。Use when: 需要统一执行规则、依赖管理、测试约束、ID 通信契约、实时事件顺序和技能联动关系。"
---

# Govern Engineering Rules

全局共享规则入口，不承载业务模块专属语义。

## 前置条件

- 任务会跨两个及以上 skills
- 或发现重复规则需要上移为共享约束

## 行为

1. 先读 `references/execution-rules.md`
2. 再读 `references/dependency-rules.md`
3. 再读 `references/testing-rules.md`
4. 再读 `references/id-contract.md`
5. 涉及实时链路时补读 `references/event-order-contract.md`
6. 涉及 UI 时补读 `references/ui-baseline.md`
7. 涉及多 skill 组合时补读 `references/skill-dependency-graph.md`

## 限制

- 不覆盖领域 skill 的业务边界
- 不在领域 skill 内复制全局规则文本，改为引用
- 不引入与既有规则冲突的新默认行为
