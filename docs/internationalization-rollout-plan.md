# SignalX 国际化落地方案

## 1. 目标

SignalX 计划上线美国区和新加坡区，国际化不能只停留在“翻译几个按钮”，而要覆盖：

- 前端固定 UI 文案
- 后端返回的内容文案
- 枚举/标签/状态字段的多语言展示
- 数字、时间、货币、百分比的本地化格式
- AI 摘要、新闻摘要、叙事说明等内容级文本

本方案的核心原则只有一句：

`前端负责界面国际化，后端负责内容国际化，语义字段由后端标准化。`

## 2. 基于现状的判断

当前项目已经有基础国际化能力：

- 存在中英文 `Localizable.strings`
  - [en.lproj/Localizable.strings](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Resources/Localization/en.lproj/Localizable.strings:1)
  - [zh-Hans.lproj/Localizable.strings](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Resources/Localization/zh-Hans.lproj/Localizable.strings:1)
- 项目中已有 `Language` 枚举和当前语言识别逻辑
  - [AppInfo.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/App/AppInfo.swift:13)
- 网络层已经会自动带 `T-Language-Id`
  - [AppInfo.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/App/AppInfo.swift:107)
  - [ApiProvider.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Common/Network/ApiProvider.swift:153)

这说明：

- 前端本地化框架不用重建
- 现在最需要补的是“规则”和“分层”
- 后端新增的 Insight 接口，必须从第一天就按多语言协议设计

## 3. 先上线哪些语言

### 3.1 第一阶段建议

- `en`
- `zh-Hans`

### 3.2 区域策略建议

- 美国区：默认 `en`
- 新加坡区：第一版也默认 `en`
- 中文用户：使用 `zh-Hans`

### 3.3 为什么新加坡区第一版仍建议以英文为主

- 新加坡主流产品默认英语完全成立
- 运营和审核层面更稳
- 可以减少前期翻译成本和测试复杂度
- 先把英文打磨好，比同时铺多语言更有价值

### 3.4 暂不建议第一版就上的语言

- `zh-Hant`
- `ms`
- `ta`

不是永远不做，而是第一版没必要一起做。

## 4. 责任边界

## 4.1 前端负责什么

前端负责所有固定 UI 文案：

- tab 名
- 页面标题
- section 标题
- 按钮文案
- 空状态文案
- 错误文案
- 加载态文案
- 本地提示
- 付费权益说明
- 风险标签、情绪标签、榜单名等展示名称

前端还负责：

- 语言跟随系统 / 用户手动切换
- 文案 fallback
- 时间、数字、百分比、货币格式化

一句话：

`凡是跟 View 强绑定的，前端本地化。`

## 4.2 后端负责什么

后端负责所有“内容型文本”：

- AI summary
- risk warning
- reason summary
- 新闻摘要
- 社媒摘要
- 叙事说明
- 专题分析
- push 内容模板

后端还负责：

- 统一语义字段和枚举值
- 根据 locale 返回对应语言内容
- 保证多端内容一致

一句话：

`凡是需要解释市场、需要生成或聚合的文本，后端本地化。`

## 4.3 不该怎么做

### 不要只让前端做国际化

如果 AI 摘要、新闻摘要都让前端翻译，会出现：

- 翻译质量不稳定
- 上下文丢失
- 不同端不一致
- 后续缓存和搜索都很麻烦

### 不要只让后端做国际化

如果所有按钮标题都靠接口返回，会出现：

- 页面开发效率低
- 离线态和异常态难维护
- 本地 UI 文案无法统一治理

## 5. 文案分层规则

这是本方案最重要的部分。

## 5.1 第一层：结构化语义字段

这层不直接给用户看，主要用于前端映射和逻辑判断。

例如：

```json
{
  "sentiment": "positive",
  "risk_level": "elevated",
  "narrative_type": "ai",
  "watch_reason_type": "divergence"
}
```

要求：

- 后端统一枚举值
- 前端绝不直接展示这些 raw value
- 前端基于本地化 key 做映射

这是最稳定的一层。

## 5.2 第二层：前端本地化展示文案

这层是固定展示词，由前端 `Localizable.strings` 管。

例如：

- `sentiment.positive = Positive`
- `sentiment.neutral = Neutral`
- `risk_level.elevated = Elevated Risk`
- `watch_reason.divergence = Divergence`

要求：

- 所有固定名词都走 key
- 不允许页面里直接写中文或英文常量

## 5.3 第三层：后端内容文案

这层是动态内容，由后端按 locale 返回。

例如：

- `summary`
- `reason_summary`
- `risk_warning`
- `digest_text`
- `timeline_event.summary`

这层不要再让前端做二次翻译。

## 6. 推荐的数据协议设计

## 6.1 Locale 传递规则

当前项目已经通过请求头传递 `T-Language-Id`：

- [AppInfo.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/App/AppInfo.swift:107)

建议继续保留，并补齐两个约定：

### 请求头

- `T-Language-Id: en`
- `T-Language-Id: zh-Hans`

### 可选 query 参数

对内容接口，允许增加：

- `?locale=en`
- `?locale=zh-Hans`

建议规则：

- 默认以后端读取 `T-Language-Id` 为主
- 若 query 参数存在，query 优先

这样方便服务端调试和缓存治理。

## 6.2 前后端统一 locale 取值

建议第一阶段只认这两个：

- `en`
- `zh-Hans`

后续扩展时再增加：

- `zh-Hant`
- `ms`
- `ta`

不要一上来就接受过多别名，比如：

- `en-US`
- `en-SG`
- `zh_CN`

建议服务端统一做 normalize：

- `en-US` -> `en`
- `en-SG` -> `en`
- `zh-CN` -> `zh-Hans`

## 6.3 Insight 接口字段约定

动态内容字段建议按以下方式返回。

### 推荐做法 A：直接返回当前 locale 文本

```json
{
  "sentiment": "positive",
  "risk_level": "elevated",
  "summary": "Attention is rising while sentiment remains mixed.",
  "risk_warning": "Short-term volatility may expand if follow-through weakens."
}
```

优点：

- 前端最简单
- 第一版最快

缺点：

- 如果页面要支持即时切语言，需要重新请求

### 推荐做法 B：内容接口支持多语言对象

只对极少量高价值内容字段使用：

```json
{
  "sentiment": "positive",
  "summary_i18n": {
    "en": "Attention is rising while sentiment remains mixed.",
    "zh-Hans": "当前讨论热度上升，但情绪仍有分歧。"
  }
}
```

优点：

- 前端切语言时可不重拉

缺点：

- 响应变重
- 缓存复杂度升高

建议：

- 第一版默认用做法 A
- 不要把所有内容字段都做成 `*_i18n`

## 6.4 枚举字段约定

后端统一给 raw enum，前端本地化展示。

例如：

```json
{
  "sentiment": "positive",
  "sentiment_bias": "bullish",
  "risk_level": "elevated",
  "event_type": "attention_spike",
  "source_type": "news"
}
```

前端映射到 key：

- `sentiment.positive`
- `sentiment_bias.bullish`
- `risk_level.elevated`
- `event_type.attention_spike`
- `source_type.news`

## 7. 前端本地化规则

## 7.1 新增文案必须走 key

本项目从现在开始新增文案统一要求：

- 不直接写 `"市场洞察"`
- 不直接写 `"AI 洞察摘要"`
- 不直接写 `"风险提示"`

而是：

- `Text("insight.market_overview.title")`
- `Text("insight.asset_summary.title")`
- `Text("insight.risk_warning.title")`

## 7.2 key 命名规则

推荐采用：

- `tab.*`
- `common.*`
- `insight.*`
- `market.*`
- `asset.*`
- `discover.*`
- `subscription.*`
- `error.*`
- `empty.*`

例如：

- `insight.market_overview.title`
- `insight.timeline.title`
- `market.watchlist.title`
- `asset.summary.risk_warning`

## 7.3 不要把动态内容写进 strings

不要把下面这些塞进 `Localizable.strings`：

- 币种洞察摘要全文
- 新闻摘要全文
- 社媒摘要全文
- AI 风险提示全文

这些应由后端按 locale 返回。

## 7.4 日期数字格式化

前端统一根据当前 locale 做格式化：

- 百分比
- 金额
- 大数缩写
- 日期时间

特别注意：

- 美国区：`1,234.56`
- 中文：`1,234.56` 也可，但日期格式不同
- 时间展示建议统一用产品样式，不必完全依赖系统长日期

## 8. 后端实现规则

## 8.1 后端新增 Insight 接口必须支持 locale

例如：

- `/api/v1/insight/market/overview`
- `/api/v1/insight/assets/{instrument_id}/summary`
- `/api/v1/insight/assets/{instrument_id}/timeline`
- `/api/v1/insight/assets/{instrument_id}/digest`

都应支持：

- 读取 `T-Language-Id`
- 统一 fallback 到 `en`

## 8.2 fallback 规则

推荐后端 fallback：

1. 请求语言
2. `en`
3. 空字符串或默认占位

不要 fallback 到中文。

因为美国区和新加坡区上线时，英文应是默认安全语言。

## 8.3 枚举值永远不要本地化

后端不要返回：

- `"sentiment": "偏多"`
- `"risk_level": "风险较高"`

而要返回：

- `"sentiment": "positive"`
- `"risk_level": "elevated"`

否则前端无法统一多语言，也无法做稳定逻辑判断。

## 8.4 AI / 摘要类内容生成建议

生成型内容要按 locale 直接生成，而不是先生成中文再翻译。

推荐流程：

1. 根据结构化数据生成内容
2. 以目标 locale 输出
3. 缓存按 `resource + locale` 维度存储

例如缓存 key：

- `asset_summary:BTC-USDT:en`
- `asset_summary:BTC-USDT:zh-Hans`

## 9. 前后端字段约定模板

下面给一套可以直接对齐的模板。

## 9.1 市场总览接口

```json
{
  "sentiment": "neutral",
  "risk_appetite": "cooling",
  "headline_narratives": [
    {
      "id": "ai",
      "name": "AI",
      "heat_score": 82,
      "heat_change_pct": 18.2
    }
  ],
  "summary": "Market sentiment is cautious, with focus centered on AI and meme themes.",
  "updated_at": "2026-04-19T08:00:00Z"
}
```

约定：

- `sentiment`、`risk_appetite` 是结构化枚举
- `summary` 是后端内容文案
- `name` 若是固定叙事名，建议返回标准英文值，前端可按 key 映射

## 9.2 币种详情摘要接口

```json
{
  "instrument_id": "BTC-USDT",
  "sentiment": "neutral",
  "risk_level": "moderate",
  "narrative_tags": ["macro", "core_asset"],
  "summary": "Discussion remains stable while directional conviction is limited.",
  "risk_warning": "Volatility may expand if macro headlines intensify."
}
```

约定：

- `narrative_tags` 返回标准 tag
- 前端映射成显示词
- `summary` / `risk_warning` 由后端按 locale 返回

## 9.3 时间线事件接口

```json
{
  "items": [
    {
      "event_id": "evt_001",
      "event_type": "attention_spike",
      "timestamp": "2026-04-19T08:15:00Z",
      "title": "Attention picked up",
      "summary": "Mentions accelerated after a fresh narrative catalyst.",
      "sentiment_direction": "positive",
      "impact_level": "medium"
    }
  ]
}
```

约定：

- `event_type`、`sentiment_direction`、`impact_level` 是枚举
- `title`、`summary` 是内容文案

## 10. 前端落地建议

## 10.1 先缩减支持语言集合

当前 `Language` 枚举很大：

- [AppInfo.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/App/AppInfo.swift:13)

但项目实际只有 `en` 和 `zh-Hans` 资源。

建议第一阶段明确产品支持集：

- `en`
- `zh-Hans`

其他语言不要在产品设置中暴露，避免“可选但无内容”。

## 10.2 新增一个 locale 适配层

建议前端新增统一访问点：

- `AppLocale.current`
- `AppLocale.apiLocale`
- `AppLocale.displayLocale`

职责：

- 规范前端显示语言
- 规范接口请求语言
- 统一 fallback

## 10.3 Insight 模块新增 strings namespace

建议补充：

- `insight.market_overview.title`
- `insight.narratives.title`
- `insight.watchlist.title`
- `insight.core_assets.title`
- `insight.digest.title`
- `asset.insight_summary.title`
- `asset.metrics.title`
- `asset.timeline.title`
- `asset.social_digest.title`

## 11. 测试与验收

## 11.1 前端验收

至少验证：

- 系统语言英文时，固定 UI 是否完整英文
- 系统语言中文时，固定 UI 是否完整中文
- 未翻译 key 是否能被及时发现
- 页面切换和下拉刷新后语言是否一致

## 11.2 后端验收

至少验证：

- 相同资源在 `en` 和 `zh-Hans` 下内容不同
- 非支持语言能 fallback 到 `en`
- 枚举值不因语言变化而变化

## 11.3 联调验收

重点看：

- 标题是前端翻译
- 摘要是后端翻译
- 标签是前端映射
- 时间数字格式符合当前语言预期

## 12. 最终建议

针对 SignalX 这次美国区和新加坡区上线，建议你们马上统一这 6 条规则：

1. 第一版只正式支持 `en` 和 `zh-Hans`
2. 所有固定 UI 文案由前端本地化
3. 所有动态内容文案由后端按 locale 返回
4. 所有枚举和标签字段都返回稳定 raw value，不返回本地化展示词
5. locale 统一使用 `T-Language-Id`，必要时可加 `locale` query 参数覆盖
6. 后端 fallback 默认到 `en`，不要默认回中文

如果按这套方案执行，后面无论你们是继续做美国区，还是扩新加坡区更多语言，都不会返工核心协议。
