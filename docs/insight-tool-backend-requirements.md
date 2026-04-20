# SignalX Insight Tool 改造：前端推导后端需求

## 1. 目标与结论

本次不是重做一个新 App，而是将现有 `SignalX` 从“行情/信号工具”重构为“Crypto Insight Tool”。

前端改造方向已经很明确：

- 首页先回答“市场发生了什么”
- 币种详情先回答“为什么值得关注”
- K 线退到证据层
- 信号能力去交易化，转成情绪、热度、风险提示体系

基于现有代码，结论很清晰：

- 行情实时能力、K 线能力、K 线 marker 能力已有基础，可以复用
- 当前后端主要提供的是“价格/信号/历史 K 线”数据
- 真正缺的是“洞察层数据”：市场总览、叙事、情绪、热度、摘要、事件时间线、发现页内容
- 因此这次应从前端页面结构反推一层新的 Insight API，而不是继续把现有 `signal` 接口硬改成首页和详情页的数据源

## 2. 现有前端与接口现状

### 2.1 当前信息架构

当前 Tab 在 [Tabs.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/App/Tabs.swift:10)：

- `markets`
- `analytics`
- `signals`
- `alerts`
- `profile`
- `strategies`

当前页面心智仍然偏旧：

- 市场页 [MarketsView.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/MarketsView.swift:10) 只有“币种”列表
- 首页本质就是币种列表 [CoinListView.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Coins/CoinListView.swift:11)
- 信号页标题是“精选信号” [SignalsView.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Signals/SignalsView.swift:44)
- 币种详情第一屏仍然是头部行情 + 周期切换 + K 线 [CoinDetailView.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Detail/CoinDetailView.swift:17)

### 2.2 当前已有后端能力

#### A. 币种/行情

接口定义在 [CoinsApi.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Endpoint/CoinsApi.swift:9)：

- `GET /api/v1/instruments/all`
- `GET /api/v1/market/sorted-inst-ids`
- `POST /api/v1/market/detail`

已支撑的数据能力：

- 全量币种元数据
- 排序后的币种 ID 列表
- 币种详情历史 K 线
- 币种详情历史信号 marker

#### B. 实时能力

实时列表和实时 K 线主要靠 WebSocket：

- 排序推送、分页交易对数据、ticker 批量更新 [TickerDataManager.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Coins/TickerDataManager.swift:10)
- 支持 `get_page`、`change_sort`、`subscribe_candle`、`unsubscribe_candle` [WSMessage.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Coins/WSMessage.swift:8)

现有可复用能力：

- 币种列表实时价格
- 核心资产实时刷新
- K 线实时更新
- K 线上的事件标记点渲染

#### C. 信号

接口定义在 [SignalsApi.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Endpoint/SignalsApi.swift:9)：

- `GET /api/v1/signal/list`
- `GET /api/v1/signal/detail`
- `POST /api/v1/signal/execute`

当前信号模型 [Signal.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Signals/Signal.swift:9) 仍然强交易导向：

- `command`
- `entry_price`
- `recommended_sl`
- `recommended_tp`
- `signal_execute`

这套接口可以作为“底层计算来源”继续存在，但不适合直接承担 Insight 首页和新版详情页的数据协议。

### 2.3 当前已有的 Insight 雏形

这部分很重要，说明不用从零开始：

- 币种详情已支持 `history_signals` 作为图表 marker 数据 [CoinDetail.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Detail/CoinDetail.swift:8)
- 详情页 ViewModel 会把历史信号转成 K 线 marker [CoinDetailViewModel.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Detail/CoinDetailViewModel.swift:376)
- 图表层已支持 marker 更新 [CoinDetailChartView.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Detail/CoinDetailChartView.swift:34)

这意味着“事件点叠加”和“时间线映射到图表”已经有了很好的前端承接点，后端只要把 marker 语义从 `signal` 扩展到 `insight/event` 即可。

## 3. 从页面改造反推数据需求

## 3.1 首页：Market Overview

目标：回答 3 个问题

- 今天市场整体情绪怎么样？
- 今天大家在讨论什么？
- 哪些币值得点进去看？

### 前端需要的数据模块

#### A. 市场总览 Hero

前端需要：

- 市场情绪 `bullish | neutral | bearish`
- 风险偏好 `warming | cooling | stable`
- 今日主叙事列表
- AI 一句话摘要
- 市场摘要更新时间
- 可选：BTC/ETH 主导作用说明

现有接口覆盖：

- 无

结论：

- 必须新增首页市场总览接口

#### B. 热点叙事卡片

前端需要：

- 叙事名称
- 热度值
- 热度变化率
- 情绪倾向
- 代表币列表
- 叙事解释文案

现有接口覆盖：

- 无

结论：

- 必须新增叙事相关接口

#### C. 值得关注的币种

前端需要按“原因”分组：

- 情绪升温
- 热度异动
- 价格与情绪背离

每个 item 至少需要：

- `instrument_id`
- `symbol`
- `price`
- `change_24h`
- `sentiment_score`
- `sentiment_change`
- `attention_score`
- `attention_change`
- `narrative_tags`
- `reason_summary`
- `watch_reason_type`

现有接口覆盖：

- 只有价格和 24h 涨跌可从 ticker 得到

结论：

- 需要新增“观察建议”列表接口，不能只靠现有涨跌排序

#### D. 轻量行情模块

前端需要：

- 少量核心资产实时行情
- 允许与“洞察列表”复用同一 instrument 基础字段

现有接口覆盖：

- 已有，直接复用 ticker + WebSocket

结论：

- 后端无需新增专门接口，可继续复用现有实时行情能力

#### E. 新闻 / X 摘要

前端需要：

- 今日摘要列表
- 高频关键词
- 来源类型 `news | x | mixed`
- 每条摘要的关联币种/关联叙事

现有接口覆盖：

- 无

结论：

- 必须新增发现/摘要类接口

## 3.2 市场页：Observation Hub

目标：从“全市场价格列表”转成“观察列表 + 榜单 + 热点池”。

### 前端需要的 Segment

#### A. 核心资产

需要字段：

- `instrument_id`
- `price`
- `change_24h`
- `sentiment_score`
- `attention_score`
- `tags`

现有接口覆盖：

- 价格类字段已有
- 情绪/热度没有

结论：

- 后端需要在核心资产列表接口中补充情绪与热度字段

#### B. 自选

需要字段：

- 与核心资产一致
- 额外需要 `is_watchlisted`
- 可选：提醒状态

现有接口覆盖：

- 目前项目里没有真正完整的自选行情接口

结论：

- 后端需要支持“按用户自选返回 insight 列表”

#### C. 热点池

需要的是“系统筛出来的候选集”，不是全量市场。

筛选维度至少包括：

- 提及量暴增
- 情绪快速变化
- 叙事切换明显
- 短期价格与热度背离

现有接口覆盖：

- 无

结论：

- 必须新增热点池接口

#### D. 榜单

需要多维榜单而不是单一涨跌榜：

- 涨跌榜
- 热度榜
- 情绪升温榜
- 背离榜

现有接口覆盖：

- 目前只有价格排序链路

结论：

- WebSocket 排序能力如果未来要继续复用，需要支持新的排序字段
- 如果不走 WS 排序，则至少提供榜单 REST 接口

建议：

- 榜单优先使用 REST 拉取，避免前端因为榜单太多而订阅过重

## 3.3 币种详情页：Insight First

这是这次最关键的页面。

### 前端需要的新结构

#### A. 顶部概要区

需要字段：

- `instrument_id`
- `symbol`
- `display_name`
- `price`
- `change_24h`
- `sentiment_score`
- `attention_score`
- `mention_change_24h`
- `narrative_tags`
- `risk_level`

现有接口覆盖：

- 价格类已存在
- 其余 insight 字段没有

#### B. AI 洞察摘要

需要字段：

- `summary`
- `focus_points`
- `sentiment_reason`
- `divergence_comment`
- `risk_warning`
- `updated_at`

现有接口覆盖：

- 无

结论：

- 必须新增资产级洞察摘要字段或接口

#### C. 关键指标卡片

建议后端直接给结构化数据，而不是只返回一堆原始数值：

- `sentiment_score`
- `attention_score`
- `mention_change`
- `narrative_strength`
- `price_sentiment_divergence`

每个指标最好附带：

- 当前值
- 环比变化
- 状态标签
- 一句话解释

#### D. K 线与 Insight Overlay

现有 K 线接口可以继续复用，但 marker 语义要扩展。

当前 `history_signals` 只适合交易信号点，不够支撑 Insight Tool。

建议新增或扩展为统一事件点结构：

- `event_id`
- `timestamp`
- `event_type`
- `title`
- `summary`
- `impact_level`
- `sentiment_direction`
- `related_narrative`
- `related_source_type`
- `marker_price`

事件类型建议：

- `sentiment_shift`
- `attention_spike`
- `news_event`
- `narrative_switch`
- `price_breakout`
- `risk_warning`

现有接口覆盖：

- 只有 `history_signals`

结论：

- 最少要给详情页补一个事件 marker 数据集

#### E. 事件 / 热点时间线

需要字段：

- 时间
- 事件标题
- 事件摘要
- 事件类型
- 情绪方向
- 影响强度
- 关联来源
- 是否关联到 K 线定位

现有接口覆盖：

- 无

结论：

- 必须新增详情页时间线接口

#### F. 新闻 / X / hype 摘要

需要字段：

- 摘要文本
- 来源类型
- 来源名称
- 发布时间
- 关联币种
- 关联叙事
- 情绪倾向

现有接口覆盖：

- 无

结论：

- 必须新增资产维度的内容摘要接口

## 3.4 发现页：Narrative & Digest

目标：承接新闻、X、叙事、专题分析。

### 前端第一版所需数据

- 今日关键词
- 热门叙事
- 3 条 AI 摘要

现有接口覆盖：

- 无

结论：

- 发现页需要独立数据源，不建议复用 signal/list

## 3.5 我的 / 订阅页

前端需要承接新的订阅权益：

- 完整情绪面板是否可见
- 完整币种洞察是否可见
- 热点叙事历史是否可见
- 高级榜单是否可见
- 高级提醒是否可见

现有接口覆盖：

- 当前用户与登录体系有基础
- 没有看到成体系的权益字段协议

结论：

- 后端需补“用户权限/feature gating”能力，避免前端写死

## 4. 建议给后端新增的接口清单

以下按前端落地优先级拆。

## 4.1 Phase 1 必需接口

这是你说的最小改造闭环，对产品气质改变最大。

### 1. 首页市场总览

建议：

- `GET /api/v1/insight/market/overview`

返回建议：

```json
{
  "sentiment": "neutral",
  "risk_appetite": "cooling",
  "headline_narratives": [
    {
      "id": "ai",
      "name": "AI",
      "heat_score": 82,
      "heat_change_pct": 18.2,
      "sentiment": "positive",
      "leader_assets": ["FET", "TAO"]
    }
  ],
  "summary": "市场情绪偏谨慎，热点集中在 AI 和 Meme，BTC 稳定，ETH 讨论度回落。",
  "updated_at": "2026-04-18T12:00:00Z"
}
```

### 2. 首页值得关注列表

建议：

- `GET /api/v1/insight/market/watchlist`

支持参数：

- `group=sentiment_rising|attention_spike|divergence`
- `limit`

### 3. 币种详情洞察摘要

建议：

- `GET /api/v1/insight/assets/{instrument_id}/summary`

返回建议：

```json
{
  "instrument_id": "BTC-USDT",
  "symbol": "BTC",
  "sentiment_score": 64,
  "attention_score": 71,
  "mention_change_pct": 13.5,
  "narrative_tags": ["Macro", "ETF"],
  "summary": "当前讨论热度温和上升，情绪偏中性偏多，短线缺少爆发型催化。",
  "focus_points": [
    "ETF 相关讨论稳定",
    "价格波动收窄",
    "风险偏好未明显扩张"
  ],
  "risk_warning": "若宏观风险事件升温，短线波动可能放大。",
  "updated_at": "2026-04-18T12:00:00Z"
}
```

### 4. 币种详情时间线 / 事件点

建议：

- `GET /api/v1/insight/assets/{instrument_id}/timeline`

支持参数：

- `period=24h|7d`
- `limit`

### 5. 币种详情社媒/新闻摘要

建议：

- `GET /api/v1/insight/assets/{instrument_id}/digest`

## 4.2 Phase 2 推荐接口

### 6. 叙事列表

- `GET /api/v1/insight/narratives`

支持：

- 热度排序
- 涨跌排序
- 仅显示活跃叙事

### 7. 热点池

- `GET /api/v1/insight/market/hot-assets`

### 8. 多维榜单

- `GET /api/v1/insight/rankings`

支持参数：

- `type=price_change|attention|sentiment_rising|divergence`
- `window=24h|4h|1h`

### 9. 发现页摘要流

- `GET /api/v1/insight/discover/digest`

### 10. 叙事详情 / 专题页

- `GET /api/v1/insight/narratives/{id}`

## 4.3 Phase 3 需要的用户态接口

### 11. Insight 自选列表

- `GET /api/v1/user/watchlist/insight`

### 12. 高级提醒配置

- `GET /api/v1/user/alerts/insight`
- `POST /api/v1/user/alerts/insight`

### 13. 订阅权益 / 功能开关

- `GET /api/v1/user/entitlements`

## 5. 建议优先补字段，而不是让前端拼接推导

下面这些字段如果后端不直接提供，前端会做得很脆弱。

### 5.1 市场与币种通用 Insight 字段

- `sentiment_score`
- `sentiment_change`
- `attention_score`
- `attention_change`
- `mention_count`
- `mention_change_pct`
- `narrative_tags`
- `narrative_strength`
- `risk_level`
- `divergence_score`
- `reason_summary`
- `updated_at`

原因：

- 这些都是产品语义字段，应该由后端统一口径
- 如果前端自己拼，后面首页、市场页、详情页会出现解释不一致

### 5.2 事件点字段

- `event_id`
- `event_type`
- `title`
- `summary`
- `timestamp`
- `impact_level`
- `sentiment_direction`
- `marker_price`
- `related_sources`
- `related_narrative`

原因：

- 这类数据同时服务详情时间线、K 线 overlay、摘要卡片
- 用统一结构最利于前端组件复用

### 5.3 叙事字段

- `narrative_id`
- `name`
- `heat_score`
- `heat_change_pct`
- `sentiment`
- `leader_assets`
- `summary`
- `keywords`

## 6. 现有接口建议如何复用/演进

## 6.1 `market/detail` 不建议继续只返回 K 线 + history_signals

当前 [CoinsApi.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Endpoint/CoinsApi.swift:50) 对应的 `POST /api/v1/market/detail` 已是详情聚合入口，但返回模型 [CoinDetail.swift](/Users/xiaoyuan/Documents/work/git/SignalX/SignalX/Moudles/Markets/Detail/CoinDetail.swift:8) 过于偏底层。

建议两种方案二选一：

### 方案 A：保持 `market/detail` 专注图表

只负责：

- K 线
- Volume
- marker / event overlay

另起新的 insight 接口负责摘要和时间线。

优点：

- 语义清晰
- 不会把一个接口做成超大杂烩

### 方案 B：把 `market/detail` 扩成聚合详情接口

除 K 线外，再补：

- `asset_overview`
- `insight_summary`
- `key_metrics`
- `event_timeline`
- `social_digest`

优点：

- 前端首屏请求次数少

缺点：

- 接口会很重
- 图表切周期时容易重复拉取不必要数据

建议：

- 前端体验和演进性更优的做法是方案 A

## 6.2 `signal` 体系建议保留计算，不建议保留展示话术

当前信号字段与页面绑定较深：

- `BUY / SELL`
- `recommended_sl / tp`
- `execute`

建议后端不要急着删数据能力，而是做一层展示映射：

- `buy_signal` -> `sentiment_strengthening`
- `sell_signal` -> `risk_elevated`
- `prediction` -> `insight`
- `strategy_view` -> `market_view`

也就是说：

- 算法层可以继续叫 signal
- 面向客户端展示层应新出 insight 协议

## 6.3 实时能力建议只覆盖核心资产与必要榜单

当前 WS 能力很适合：

- 核心资产
- 自选
- 详情 K 线

不建议一开始就把下面这些也做成高频实时：

- 热点池
- 叙事榜
- 发现页摘要流

建议这些先走 REST + 轮询缓存：

- 30 秒
- 1 分钟
- 5 分钟

具体按模块不同设置 TTL。

## 7. 前后端协同时最容易卡住的点

### 7.1 “情绪分/热度分”口径不统一

如果后端不统一定义，前端会出现这些问题：

- 首页和详情页同一币种分数不同
- 排行榜排序口径和卡片展示口径不同
- 文案解释冲突

建议后端统一输出：

- 分数范围
- 解释规则
- 时间窗口
- 刷新频率

### 7.2 摘要文案不稳定

Insight Tool 非常依赖一句话解释。

建议后端返回：

- `summary`
- `reason_summary`
- `risk_warning`

而不是让前端拿多个字段自己模板拼。

### 7.3 叙事与币种关系表

如果没有稳定的“叙事 <-> 币种”映射，首页的叙事卡和详情页标签会很难一致。

建议后端维护：

- 叙事 ID
- 代表币
- 相关币
- 热度变化

## 8. 给后端的最小交付版本

如果这次只想支撑你定的前三个优先级，后端最少补这 5 项：

1. `GET /api/v1/insight/market/overview`
2. `GET /api/v1/insight/market/watchlist`
3. `GET /api/v1/insight/assets/{instrument_id}/summary`
4. `GET /api/v1/insight/assets/{instrument_id}/timeline`
5. `GET /api/v1/insight/assets/{instrument_id}/digest`

这样就能支撑：

- 首页顶部 `MarketOverviewHeroCard`
- 首页“值得关注”列表
- 详情页 `AssetInsightSummaryCard`
- 详情页时间线模块
- 详情页社媒/新闻摘要模块

K 线和实时价格继续复用现有能力即可。

## 9. 推荐的前后端对齐方式

建议你和后端对齐时，不要从“接口名”开始谈，而从“页面模块”开始谈：

1. 首页 Hero 需要哪些字段
2. 首页观察建议列表需要哪些字段
3. 详情页摘要卡需要哪些字段
4. 详情页时间线和图表 marker 如何共用一套事件结构
5. 榜单和叙事是否要独立成模块

按这个顺序，对齐会比直接讨论“把 signal/detail 改一下”更顺。

## 10. 前端侧建议

即使后端还没完全补齐，前端可以先做两层抽象，避免后面返工：

### 10.1 先把展示模型改名

优先在 ViewModel / DTO 层做映射：

- `SignalViewData` -> `InsightCardViewData`
- `command` -> `sentimentLabel`
- `finalScore` -> `insightScore`

### 10.2 先把详情页拆成 5 个区块组件

即使里面先用 mock 数据，也值得先拆：

- `AssetHeaderSummary`
- `AssetInsightSummaryCard`
- `KeyMetricsGrid`
- `AssetKLineSection`
- `InsightTimelineSection`

这样后端字段补齐后，接入成本最低。

## 11. 最终判断

这次前端重构真正依赖后端补充的，不是更复杂的行情接口，而是“解释层接口”。

现有系统已经足够支撑：

- 实时价格
- K 线
- 基础 marker

真正缺的是：

- 市场总览
- 叙事
- 情绪与热度
- 事件时间线
- 新闻/X 摘要
- 用户洞察权益

所以这次后端需求应聚焦在一条新的 `insight` 数据层，而不是继续放大已有 `signal` 数据层。
