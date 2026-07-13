# Event Contract

- topic: 区块级领域主题，不使用 page.changed 这类过粗粒度
- payload 最小结构：`topic`、`entity`、`id`、`version`、`at`
- 主键统一使用 `id`
- 顺序统一遵守 `event-order-contract.md`
