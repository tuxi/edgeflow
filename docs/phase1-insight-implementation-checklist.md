# Phase 1 Insight 实施清单

## 1. 目标

这份清单用于把 Insight Phase 1 从“方向正确”推进到“可以排期开发”。

Phase 1 只追求一个目标：

- 用最小改造闭环支撑 5 个 `insight` 接口

对应接口：

- `GET /api/v1/insight/market/overview`
- `GET /api/v1/insight/market/watchlist`
- `GET /api/v1/insight/assets/{instrument_id}/summary`
- `GET /api/v1/insight/assets/{instrument_id}/timeline`
- `GET /api/v1/insight/assets/{instrument_id}/digest`

不在 Phase 1 内解决的事情：

- 高精度情绪模型
- 全量新闻/X 接入
- 用户态个性化排序
- WebSocket 化 insight 推送
- 完整 narrative 系统

## 2. Phase 1 总体原则

- 新旧链路并行，先不破坏现有 `market` 和 `signal`
- 优先产出 artifact，再接 API
- 先让数据可查、可解释、可回放
- 先用规则和聚合闭环，不依赖复杂 AI 模型
- 对缺失数据允许“降级填充”，但字段协议先稳定

## 3. Phase 1 交付物

### 3.1 数据产物

第一批必须产出的 artifact：

- `market_overview_snapshots`
- `asset_insight_snapshots`
- `asset_timeline_events`
- `asset_digest_items`
- `watchlist_candidates`
- `pipeline_run_logs`

### 3.2 API 能力

第一批必须暴露的接口：

- `market overview`
- `market watchlist`
- `asset summary`
- `asset timeline`
- `asset digest`

### 3.3 工具层能力

第一批必须具备：

- 任务注册
- pipeline 注册
- run 状态记录
- artifact 落库

## 4. Phase 1 最小数据来源

第一版先只依赖现有项目已经有基础的数据源。

### 4.1 必须复用的数据源

- OKX price / ticker
- OKX K 线
- signals
- trend snapshots
- HyperLiquid whale leaderboard / positions
- crypto instrument 基础表

现有表和模型基础：

- `crypto_instruments`：
  [edgeflow-base/internal/model/entity/crypto.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/crypto.go:46)
- `signals`：
  [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:42)
- `trend_snapshots`：
  [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:74)
- `signal_outcomes`：
  [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:120)
- `hyper_whale_leaderboard` / `hyper_whale_position`：
  [edgeflow-base/internal/model/entity/hyperliquid.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/hyperliquid.go:72)

### 4.2 Phase 1 暂时不用强依赖的数据源

- X 原始抓取
- hype 文本摘要
- 外部新闻源
- 复杂 LLM summary

这些可以后续逐步接入，不阻塞第一版。

## 5. Artifact 设计

## 5.1 `market_overview_snapshots`

用途：

- 支撑首页顶部 Hero

建议字段：

- `id`
- `snapshot_time`
- `market_sentiment`
- `risk_appetite`
- `headline_narratives_json`
- `summary`
- `leader_assets_json`
- `source_window`
- `created_at`

最小生成逻辑：

- 用 BTC / ETH / 核心资产近期价格表现
- 结合热点资产数量变化
- 结合鲸鱼活跃度变化
- 规则生成 `market_sentiment`
- 规则生成 `risk_appetite`
- 规则拼接一句 `summary`

Phase 1 接受的简化：

- `headline_narratives` 先用 tags/规则生成
- `summary` 可先模板化

## 5.2 `asset_insight_snapshots`

用途：

- 支撑详情页 summary
- 支撑 market watchlist

建议字段：

- `id`
- `instrument_id`
- `symbol`
- `sentiment_score`
- `sentiment_change`
- `attention_score`
- `attention_change`
- `mention_count`
- `mention_change_pct`
- `narrative_tags_json`
- `risk_level`
- `divergence_score`
- `reason_summary`
- `focus_points_json`
- `risk_warning`
- `snapshot_time`
- `created_at`
- `updated_at`

最小生成逻辑：

- `sentiment_score`：
  先由 signal 方向、趋势分数、近期价格方向做规则映射
- `attention_score`：
  先由价格波动、成交活跃度代理、鲸鱼动作、事件数量做规则映射
- `narrative_tags`：
  Phase 1 可先使用人工映射或币种标签
- `reason_summary`：
  用模板生成
- `focus_points`：
  用规则挑 2 到 3 条

Phase 1 接受的简化：

- `mention_count` 和 `mention_change_pct` 没有真实社媒数据时可先置为代理指标或 `null`
- 字段先占位，协议不要缺

## 5.3 `asset_timeline_events`

用途：

- 支撑详情页 timeline
- 支撑 K 线 overlay 扩展

建议字段：

- `id`
- `event_id`
- `instrument_id`
- `symbol`
- `event_type`
- `title`
- `summary`
- `impact_level`
- `sentiment_direction`
- `marker_price`
- `related_narrative`
- `related_source_type`
- `event_time`
- `source_ref_type`
- `source_ref_id`
- `created_at`

最小事件来源：

- signal 新生成
- signal 状态变化
- trend shift
- 价格异常波动
- whale position 异动

Phase 1 建议事件类型：

- `signal_created`
- `trend_shift`
- `attention_spike`
- `price_breakout`
- `whale_activity`
- `risk_warning`

最小生成逻辑：

- 从已有 signal / trend / whale 数据定时扫描
- 满足阈值就插入 event

## 5.4 `asset_digest_items`

用途：

- 支撑详情页 digest 模块

建议字段：

- `id`
- `digest_id`
- `instrument_id`
- `symbol`
- `source_type`
- `source_name`
- `title`
- `summary`
- `sentiment`
- `related_narratives_json`
- `published_at`
- `source_url`
- `raw_ref`
- `created_at`

Phase 1 简化建议：

- 如果暂时没有内容源，就先从已有信号、趋势、鲸鱼事件生成“系统 digest”
- `source_type` 先支持：
  `system | signal | whale`

这样即使没有新闻/X，也能把摘要模块先跑起来。

## 5.5 `watchlist_candidates`

用途：

- 支撑首页/市场页“值得关注”

建议字段：

- `id`
- `instrument_id`
- `symbol`
- `group_type`
- `rank_score`
- `reason_summary`
- `snapshot_ref_id`
- `window`
- `created_at`

建议的 `group_type`：

- `sentiment_rising`
- `attention_spike`
- `divergence`

最小生成逻辑：

- 基于 `asset_insight_snapshots` 周期计算榜单结果

## 5.6 `pipeline_run_logs`

用途：

- 支撑工具层可观测性

建议字段：

- `id`
- `pipeline_name`
- `run_id`
- `status`
- `started_at`
- `finished_at`
- `duration_ms`
- `processed_count`
- `success_count`
- `failed_count`
- `error_message`
- `meta_json`

## 6. Pipeline 设计

## 6.1 Pipeline A: Asset Summary Builder

职责：

- 生成 `asset_insight_snapshots`

输入：

- `crypto_instruments`
- latest ticker / recent price window
- `signals`
- `trend_snapshots`
- whale activity 聚合结果

触发频率：

- 每 5 分钟或 15 分钟

产出优先级：

- P0

依赖关系：

- 是 `watchlist` 的上游
- 是 `market overview` 的输入之一

## 6.2 Pipeline B: Timeline Event Builder

职责：

- 生成 `asset_timeline_events`

输入：

- `signals`
- `trend_snapshots`
- whale snapshots / positions
- 短周期价格变化

触发频率：

- 每 5 分钟扫描

产出优先级：

- P0

依赖关系：

- 可独立落地，不阻塞 summary

## 6.3 Pipeline C: Market Overview Builder

职责：

- 生成 `market_overview_snapshots`

输入：

- 核心资产 summary
- 当期 watchlist candidates
- 热点 tags / narratives

触发频率：

- 每 5 分钟

产出优先级：

- P0

依赖关系：

- 依赖 Asset Summary Builder

## 6.4 Pipeline D: Watchlist Builder

职责：

- 生成 `watchlist_candidates`

输入：

- `asset_insight_snapshots`

触发频率：

- 每 5 分钟

产出优先级：

- P0

依赖关系：

- 强依赖 Asset Summary Builder

## 6.5 Pipeline E: Digest Builder

职责：

- 生成 `asset_digest_items`

输入：

- `asset_timeline_events`
- `signals`
- whale activity

触发频率：

- 每 10 分钟或 30 分钟

产出优先级：

- P1

依赖关系：

- 可以在 summary/timeline 后补

## 7. 接口与 Artifact 映射

### 7.1 `GET /api/v1/insight/market/overview`

读取：

- 最新 `market_overview_snapshots`

可选补充：

- 核心资产当前价格

### 7.2 `GET /api/v1/insight/market/watchlist`

读取：

- 最新 `watchlist_candidates`
- 关联 `asset_insight_snapshots`

### 7.3 `GET /api/v1/insight/assets/{instrument_id}/summary`

读取：

- 最新 `asset_insight_snapshots`

### 7.4 `GET /api/v1/insight/assets/{instrument_id}/timeline`

读取：

- `asset_timeline_events`

### 7.5 `GET /api/v1/insight/assets/{instrument_id}/digest`

读取：

- `asset_digest_items`

## 8. 研发任务拆分

建议按 3 条线并行推进。

### 8.1 数据引擎线

- 新增 artifact 模型
- 新增 pipeline 注册表
- 新增 pipeline run log
- 实现 Asset Summary Builder
- 实现 Timeline Event Builder
- 实现 Market Overview Builder
- 实现 Watchlist Builder
- 实现 Digest Builder

### 8.2 API 接入线

- 新增 `insight` 路由分组
- 新增 dao 查询层
- 新增 service 聚合层
- 新增 5 个 Phase 1 handler

### 8.3 协议与验证线

- 固定返回字段
- 补 mock 数据样例
- 明确空值策略
- 明确更新时间策略

## 9. 推荐开发顺序

最推荐的顺序如下：

1. 先建表和 artifact model。
2. 先做 `asset_insight_snapshots`。
3. 再做 `watchlist_candidates`。
4. 再做 `market_overview_snapshots`。
5. 并行做 `asset_timeline_events`。
6. 最后做 `asset_digest_items`。
7. 再由 `edgeflow` 接入查询 API。

原因：

- `asset summary` 是核心上游
- `watchlist` 和 `overview` 都依赖它
- `timeline` 可以独立补齐
- `digest` 对内容源依赖更强，适合放后

## 10. 空值与降级策略

Phase 1 必须明确：允许部分字段先降级，不要因为“数据还不完美”阻塞接口上线。

建议规则：

- `mention_count` 没有真实值时允许 `null`
- `mention_change_pct` 没有真实值时允许 `null`
- `narrative_tags` 没有模型时先返回空数组
- `digest` 没有外部内容时返回系统摘要
- `summary` 缺少复杂模型时先返回模板化文案

重点是：

- 字段协议稳定
- 更新时间可信
- 数据来源可解释

## 11. 风险点

### 11.1 最大风险

- 试图一次把真实社媒、叙事系统、AI 摘要全部补齐

这会直接拖慢 Phase 1。

### 11.2 第二风险

- 在 `edgeflow` API 层临时拼大量洞察逻辑

这会让系统继续变重，并模糊边界。

### 11.3 第三风险

- 不做 run log 和可观测性

这样 insight 数据出问题时会很难排查。

## 12. 完成标准

当下面 6 件事成立时，Phase 1 可以算完成：

1. `edgeflow-base` 能稳定产出 5 类 artifact。
2. `edgeflow` 能稳定暴露 5 个 `insight` 接口。
3. 首页 Hero 可以完整读 `market overview`。
4. 详情页 summary / timeline / digest 都能读取新接口。
5. artifact 生成过程有 run log 可追踪。
6. 新旧 `market/signal` 逻辑未被破坏。

## 13. 一句话执行建议

Phase 1 的正确打法不是“先做最聪明的 insight”，而是：

- 先定义产物
- 再跑通 pipeline
- 再接 API
- 最后持续提升数据质量

先让系统具备 Insight 数据层，比先把每个分数字段做到完美更重要。
