# Phase 1 Narrative Tags 口径说明

这份文档用于定义 Phase 1 的 `narrative_tags` 口径。

目标：

- 让前端拿到稳定 raw value
- 让后端在没有完整 narrative 系统时也能产出可用标签
- 避免把 instrument 原始 tag 直接透传给客户端

当前代码依据：

- [edgeflow-base/internal/insight/builder/builder.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/insight/builder/builder.go:1)
- [edgeflow/internal/service/insight_content.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight_content.go:1)

## 1. 当前结论

当前 `narrative_tags` 已经收紧成白名单口径，不再直接透传 instrument 原始 tag。

当前正式支持的 raw value 只有：

- `core_asset`
- `layer2`
- `ai`
- `meme`
- `defi`

前端应把这些值当作稳定枚举处理，而不是展示用文本。

## 2. 当前数据现实

本地库当前状态：

- `crypto_tags` 为空
- `instrument_tag_relations` 为空

这意味着当前 Phase 1 不能依赖真实 tag 表来驱动叙事系统。

因此当前后端采用的是三层策略：

1. 先尝试把 instrument tag 归一化到白名单
2. 如果没有 tag，就按 symbol 做规则映射
3. 如果仍然无法命中，就 fallback 到 `core_asset`

## 3. Phase 1 白名单

当前允许返回的值：

### 3.1 `core_asset`

含义：

- 核心大盘资产
- 或当前没有更细 narrative 时的默认分类

常见 symbol：

- `BTC`
- `ETH`
- `SOL`
- `BNB`
- `XRP`
- `ADA`
- `OKB`
- `BCH`
- `AVAX`
- `DOT`
- `TRX`
- `LTC`

### 3.2 `layer2`

含义：

- 二层扩容 / rollup / L2 相关资产

当前规则映射示例：

- `ARB`
- `OP`
- `STRK`
- `ZK`
- `ZRO`
- `LINEA`
- `MANTA`
- `METIS`
- `IMX`

### 3.3 `ai`

含义：

- AI / agent / 算力 / AI 叙事相关资产

当前规则映射示例：

- `FET`
- `TAO`
- `RENDER`
- `RNDR`
- `AIXBT`
- `VIRTUAL`
- `WLD`
- `GRASS`
- `AI16Z`

### 3.4 `meme`

含义：

- Meme / 社区炒作型资产

当前规则映射示例：

- `DOGE`
- `SHIB`
- `PEPE`
- `BONK`
- `WIF`
- `FLOKI`
- `MEMEFI`
- `PUMP`

### 3.5 `defi`

含义：

- DeFi / DEX / 借贷 / 协议类资产

当前规则映射示例：

- `AAVE`
- `UNI`
- `1INCH`
- `CRV`
- `SUSHI`
- `COMP`
- `MKR`
- `SNX`
- `LDO`
- `DYDX`
- `LINK`

## 4. 当前归一化规则

如果未来 `crypto_tags` 有值，后端会优先尝试把原始 tag 归一化到上面的白名单。

当前支持的常见 alias：

- `core`, `coreasset`, `majors`, `bluechip` -> `core_asset`
- `l2`, `layer2`, `rollup` -> `layer2`
- `ai`, `agent`, `agents` -> `ai`
- `meme`, `memecoin`, `memecoins` -> `meme`
- `defi`, `dex`, `lending` -> `defi`

不在白名单 alias 内的值：

- 不会直接透传到前端

## 5. 当前 fallback 规则

如果：

- 没有 tag 数据
- 或 tag 无法归一化
- 或 symbol 没有命中规则映射

则：

- 返回 `["core_asset"]`

这也是为什么当前很多资产仍然会落在 `core_asset`。

## 6. 前端处理建议

## 6.1 不要直接展示 raw value

前端不要直接把这些值展示给用户：

- `core_asset`
- `layer2`
- `ai`
- `meme`
- `defi`

应该用本地映射展示名称。

## 6.2 当前推荐展示映射

- `core_asset` -> 核心资产 / Core Assets
- `layer2` -> 二层扩容 / Layer 2
- `ai` -> AI 叙事 / AI
- `meme` -> Meme 板块 / Meme
- `defi` -> DeFi / DeFi

## 6.3 当前使用建议

当前 `narrative_tags` 更适合用于：

- section 标签
- 卡片标签
- timeline 的 `related_narrative`
- Hero narrative chips

还不适合用于：

- 精细 narrative 详情页
- 多层级 narrative 分析
- 强排序逻辑

## 7. 当前限制

当前 Phase 1 的 narrative 还是占位策略，主要限制有：

1. 没有真实 tag 表数据
2. 没有 narrative strength
3. 没有 narrative heat change
4. 没有 narrative graph / related assets 图谱
5. 仍然大量依赖 symbol 规则映射

所以当前最重要的价值是：

- 统一口径
- 稳定协议
- 先让前端能开发

而不是：

- 追求 narrative 的高精度分类

## 8. 下一步建议

如果后面继续迭代 BASE-16，建议顺序是：

1. 给 `crypto_tags` / `instrument_tag_relations` 补真实数据
2. 增加 `narrative_strength`
3. 增加 `narrative heat change`
4. 把 `insight/narratives` 做成独立接口
5. 再做 narrative detail / discover 流
