# Phase 1 Insight Backlog

## 1. 说明

这份 backlog 把 Phase 1 的 Insight 落地任务拆成三条主线：

- `DB / 表结构`
- `edgeflow-base`
- `edgeflow`

目标是让任务能直接进入排期，而不是继续停留在方案层。

任务拆分原则：

- 每个任务都尽量对应一个清晰交付物
- 尽量避免“超大任务”
- 优先保持新旧系统并行
- 先打通数据产物，再接 API

优先级定义：

- `P0`：Phase 1 阻塞项，必须先做
- `P1`：Phase 1 重要项，建议本期完成
- `P2`：可并行或可后置优化

状态建议：

- `Todo`
- `Doing`
- `Done`
- `Blocked`

### 当前状态说明

下面的状态已经按当前代码和联调结果回填过一版。

- `Done`：代码、迁移、真实接口或真实写库已验证通过
- `Doing`：主链路已可用，但质量、文档化或协议细节还在继续补
- `Todo`：尚未开始，或仍停留在文档目标

## 2. 开发总顺序

推荐顺序：

1. 先做 `DB / 表结构`
2. 再做 `edgeflow-base` 的 artifact 和 pipeline
3. 最后接 `edgeflow` 的查询接口

原因：

- `edgeflow-base` 需要先有落库目标
- `edgeflow` 需要先有稳定 artifact 才适合接 API

## 3. DB / 表结构 Backlog

### DB-01 新增 `market_overview_snapshots` 表

- 优先级：`P0`
- 状态：`Done`
- 目标：存储首页 Hero 所需市场概览快照
- 主要字段：
  `snapshot_time`, `market_sentiment`, `risk_appetite`, `headline_narratives_json`, `summary`, `leader_assets_json`, `source_window`
- 依赖：无
- 验收标准：
  表结构确定，可支持按最新时间读取一条快照

### DB-02 新增 `asset_insight_snapshots` 表

- 优先级：`P0`
- 状态：`Done`
- 目标：存储单资产 insight summary 快照
- 主要字段：
  `instrument_id`, `symbol`, `sentiment_score`, `sentiment_change`, `attention_score`, `attention_change`, `mention_count`, `mention_change_pct`, `narrative_tags_json`, `risk_level`, `divergence_score`, `reason_summary`, `focus_points_json`, `risk_warning`, `snapshot_time`
- 依赖：无
- 验收标准：
  可按 `instrument_id + snapshot_time` 查询最近一条快照

### DB-03 新增 `asset_timeline_events` 表

- 优先级：`P0`
- 状态：`Done`
- 目标：存储详情页 timeline 和 K 线 overlay 事件
- 主要字段：
  `event_id`, `instrument_id`, `symbol`, `event_type`, `title`, `summary`, `impact_level`, `sentiment_direction`, `marker_price`, `related_narrative`, `related_source_type`, `event_time`, `source_ref_type`, `source_ref_id`
- 依赖：无
- 验收标准：
  可按资产和时间倒序分页读取事件

### DB-04 新增 `asset_digest_items` 表

- 优先级：`P1`
- 状态：`Done`
- 目标：存储详情页 digest 项
- 主要字段：
  `digest_id`, `instrument_id`, `symbol`, `source_type`, `source_name`, `title`, `summary`, `sentiment`, `related_narratives_json`, `published_at`, `source_url`, `raw_ref`
- 依赖：无
- 验收标准：
  可按资产和发布时间倒序读取 digest

### DB-05 新增 `watchlist_candidates` 表

- 优先级：`P0`
- 状态：`Done`
- 目标：存储首页/市场页“值得关注”候选集
- 主要字段：
  `instrument_id`, `symbol`, `group_type`, `rank_score`, `reason_summary`, `snapshot_ref_id`, `window`
- 依赖：`asset_insight_snapshots`
- 验收标准：
  可按 `group_type + rank_score` 读取榜单

### DB-06 新增 `pipeline_run_logs` 表

- 优先级：`P0`
- 状态：`Done`
- 目标：记录 pipeline 运行日志
- 主要字段：
  `pipeline_name`, `run_id`, `status`, `started_at`, `finished_at`, `duration_ms`, `processed_count`, `success_count`, `failed_count`, `error_message`, `meta_json`
- 依赖：无
- 验收标准：
  每条 pipeline run 都能落库并可追查成功/失败

### DB-07 为 Phase 1 新表补索引

- 优先级：`P0`
- 状态：`Done`
- 目标：保障查询效率
- 建议索引：
  `asset_insight_snapshots(instrument_id, snapshot_time desc)`
  `asset_timeline_events(instrument_id, event_time desc)`
  `asset_digest_items(instrument_id, published_at desc)`
  `watchlist_candidates(group_type, rank_score desc)`
  `pipeline_run_logs(pipeline_name, started_at desc)`
- 依赖：`DB-01` 到 `DB-06`
- 验收标准：
  所有关键读路径都有明确索引

### DB-08 明确 JSON 字段存储方案

- 优先级：`P1`
- 状态：`Done`
- 目标：统一数组/对象字段存储口径
- 关注字段：
  `headline_narratives_json`, `leader_assets_json`, `narrative_tags_json`, `focus_points_json`, `related_narratives_json`, `meta_json`
- 依赖：`DB-01` 到 `DB-06`
- 验收标准：
  所有 JSON 字段类型、默认值和空值策略明确

### DB-09 输出 Phase 1 migration 脚本

- 优先级：`P0`
- 状态：`Done`
- 目标：形成可执行 migration
- 依赖：`DB-01` 到 `DB-08`
- 验收标准：
  开发和测试环境能一致创建 Phase 1 所有表

## 4. edgeflow-base Backlog

### BASE-01 新增 insight 模块目录骨架

- 优先级：`P0`
- 状态：`Done`
- 目标：在 `edgeflow-base` 内建立独立的 insight 领域模块
- 建议包含：
  `internal/insight/model`
  `internal/insight/repository`
  `internal/insight/pipeline`
  `internal/insight/builder`
  `internal/insight/task`
- 依赖：无
- 验收标准：
  insight 代码不散落进旧 `signal`/`hyper` 逻辑里

### BASE-02 设计 artifact model

- 优先级：`P0`
- 状态：`Done`
- 目标：为 5 类 artifact 建立领域模型
- 涵盖：
  market overview, asset summary, timeline event, digest item, watchlist candidate, pipeline run log
- 依赖：`DB-01` 到 `DB-06`
- 验收标准：
  model 与表结构一一对应，字段口径统一

### BASE-03 设计 insight repository 接口

- 优先级：`P0`
- 状态：`Done`
- 目标：抽象 artifact 落库和查询接口
- 依赖：`BASE-02`
- 验收标准：
  `save latest snapshot / append event / append digest / save run log` 这些动作有统一 repository 接口

### BASE-04 实现 pipeline run logger

- 优先级：`P0`
- 状态：`Done`
- 目标：给每条 pipeline 增加运行日志记录
- 依赖：`DB-06`, `BASE-03`
- 验收标准：
  成功/失败/耗时/处理数量都能记录

### BASE-05 实现 pipeline registry

- 优先级：`P0`
- 状态：`Done`
- 目标：统一注册 Phase 1 的各条 pipeline
- 依赖：`BASE-01`
- 验收标准：
  不再把所有 insight 任务直接堆在 `main.go`

### BASE-06 实现 task scheduler 接入层

- 优先级：`P0`
- 状态：`Done`
- 目标：把 pipeline registry 接到现有 cron 体系
- 依赖：`BASE-05`
- 验收标准：
  可按配置注册并启动 pipeline 任务

### BASE-07 实现 Asset Summary Builder

- 优先级：`P0`
- 状态：`Done`
- 目标：产出 `asset_insight_snapshots`
- 输入：
  ticker/K 线、signals、trend snapshots、whale 数据、instrument 基础数据
- 依赖：`BASE-02`, `BASE-03`, `BASE-06`
- 验收标准：
  核心资产能稳定写入 summary 快照

### BASE-08 明确 `sentiment_score` 规则版算法

- 优先级：`P0`
- 状态：`Doing`
- 目标：给第一版情绪分一个稳定、可解释的规则
- 建议输入：
  signal direction、trend final score、近周期价格方向
- 依赖：`BASE-07`
- 验收标准：
  有文档化规则，可复现，可调参

### BASE-09 明确 `attention_score` 规则版算法

- 优先级：`P0`
- 状态：`Doing`
- 目标：给第一版热度分一个稳定、可解释的规则
- 建议输入：
  波动率代理、事件数量、whale 活跃度、榜单变化
- 依赖：`BASE-07`
- 验收标准：
  有文档化规则，可复现，可调参

### BASE-10 实现 `reason_summary` 模板生成

- 优先级：`P1`
- 状态：`Done`
- 目标：为 asset summary 生成解释文案
- 当前进度：
  已补一版更贴近产品表达的规则型 `reason_summary / summary / digest` 文案，降低系统模板感
- 依赖：`BASE-07`, `BASE-08`, `BASE-09`
- 验收标准：
  每条 snapshot 都能产出一句合理摘要

### BASE-11 实现 Timeline Event Builder

- 优先级：`P0`
- 状态：`Done`
- 目标：产出 `asset_timeline_events`
- 输入：
  signals、trend shift、价格异常、whale activity
- 依赖：`BASE-02`, `BASE-03`, `BASE-06`
- 验收标准：
  关键资产能产出时间线事件

### BASE-12 定义 Phase 1 事件类型枚举

- 优先级：`P0`
- 状态：`Doing`
- 目标：固定 Phase 1 事件分类
- 建议值：
  `signal_created`, `trend_shift`, `attention_spike`, `price_breakout`, `whale_activity`, `risk_warning`
- 当前进度：
  已补事件类型说明文档，但当前代码实际只产出 `signal_created`, `whale_activity`, `risk_warning`
- 依赖：`BASE-11`
- 验收标准：
  API 和 builder 共用同一套事件类型口径

### BASE-13 实现 Watchlist Builder

- 优先级：`P0`
- 状态：`Done`
- 目标：产出 `watchlist_candidates`
- 分组：
  `sentiment_rising`, `attention_spike`, `divergence`
- 依赖：`BASE-07`
- 验收标准：
  能稳定生成三个分组的候选列表

### BASE-14 实现 Market Overview Builder

- 优先级：`P0`
- 状态：`Done`
- 目标：产出 `market_overview_snapshots`
- 输入：
  核心资产 summary、watchlist、热点标签、whale 活跃度
- 依赖：`BASE-07`, `BASE-13`
- 验收标准：
  首页 Hero 需要的字段都有对应快照

### BASE-15 实现 Digest Builder

- 优先级：`P1`
- 状态：`Done`
- 目标：产出 `asset_digest_items`
- Phase 1 先从系统内数据生成 digest
- 依赖：`BASE-11`
- 验收标准：
  没有外部新闻/X 时，详情页也能返回系统 digest

### BASE-16 设计 narrative tag 的 Phase 1 占位策略

- 优先级：`P1`
- 状态：`Done`
- 目标：在没有完整叙事系统前先稳定返回 `narrative_tags`
- 建议来源：
  instrument tags、人工映射、规则映射
- 当前进度：
  已收紧为稳定白名单 `core_asset`, `layer2`, `ai`, `meme`, `defi`，并补了口径说明文档
- 依赖：`BASE-07`
- 验收标准：
  `narrative_tags` 字段不为空协议，不阻塞 API

### BASE-17 设计空值与降级策略实现

- 优先级：`P0`
- 状态：`Done`
- 目标：确保缺失数据不阻塞 pipeline
- 重点字段：
  `mention_count`, `mention_change_pct`, `narrative_tags`, `digest`
- 依赖：`BASE-07`, `BASE-14`, `BASE-15`
- 验收标准：
  pipeline 在缺少社媒/内容源时仍能稳定产出

### BASE-18 新增 pipeline 健康检查/监控日志

- 优先级：`P1`
- 状态：`Todo`
- 目标：在运行日志之外补基础可观察性
- 依赖：`BASE-04`
- 验收标准：
  能快速看出哪条 pipeline 最近失败

### BASE-19 抽象 message bus 接口

- 优先级：`P2`
- 状态：`Todo`
- 目标：为后续 Kafka/Redis MQ 统一做准备
- 依赖：无
- 验收标准：
  insight pipeline 不直接依赖某一种总线实现

### BASE-20 为 Phase 1 pipeline 输出 mock / seed 数据

- 优先级：`P1`
- 状态：`Todo`
- 目标：帮助 API 联调和前端联调
- 依赖：`BASE-07` 到 `BASE-15`
- 验收标准：
  在真实数据不完整时也能联调接口

## 5. edgeflow Backlog

### API-01 新增 `insight` 路由分组

- 优先级：`P0`
- 状态：`Done`
- 目标：在 router 中增加 `insight/*` 路由入口
- 依赖：无
- 验收标准：
  能独立挂载 `insight` 相关 handler

### API-02 新增 insight model

- 优先级：`P0`
- 状态：`Done`
- 目标：定义对外返回的 `insight` 响应结构
- 涵盖：
  overview, watchlist item, asset summary, timeline item, digest item
- 依赖：上游 artifact 字段稳定
- 验收标准：
  API 模型与产品协议一致

### API-03 新增 insight dao

- 优先级：`P0`
- 状态：`Done`
- 目标：查询 Phase 1 artifact 表
- 依赖：`DB-01` 到 `DB-06`
- 验收标准：
  有独立 dao，不复用旧 `signal` dao 去硬查

### API-04 新增 insight service

- 优先级：`P0`
- 状态：`Done`
- 目标：封装 Phase 1 的查询聚合逻辑
- 依赖：`API-03`
- 验收标准：
  handler 层只做参数处理，不写聚合逻辑

### API-05 实现 `GET /api/v1/insight/market/overview`

- 优先级：`P0`
- 状态：`Done`
- 目标：返回首页 Hero 数据
- 依赖：`API-04`, `BASE-14`
- 验收标准：
  可返回最新 market overview 快照

### API-06 实现 `GET /api/v1/insight/market/watchlist`

- 优先级：`P0`
- 状态：`Done`
- 目标：返回首页/市场页候选列表
- 依赖：`API-04`, `BASE-13`
- 验收标准：
  支持 `group` 和 `limit`

### API-07 实现 `GET /api/v1/insight/assets/{instrument_id}/summary`

- 优先级：`P0`
- 状态：`Done`
- 目标：返回详情页顶部 summary
- 依赖：`API-04`, `BASE-07`
- 验收标准：
  返回 summary、focus points、risk warning 等核心字段

### API-08 实现 `GET /api/v1/insight/assets/{instrument_id}/timeline`

- 优先级：`P0`
- 状态：`Done`
- 目标：返回详情页时间线
- 依赖：`API-04`, `BASE-11`
- 当前进度：
  `period` 参数已收紧为 `24h | 7d`，默认 `7d`，并已接入真实时间过滤
- 验收标准：
  支持 `period`、`limit`

### API-09 实现 `GET /api/v1/insight/assets/{instrument_id}/digest`

- 优先级：`P1`
- 状态：`Done`
- 目标：返回详情页 digest
- 依赖：`API-04`, `BASE-15`
- 验收标准：
  在没有外部新闻源时也能返回系统 digest

### API-10 定义 API 空值策略

- 优先级：`P0`
- 状态：`Done`
- 目标：避免前端因字段缺失崩溃
- 重点字段：
  `mention_count`, `mention_change_pct`, `narrative_tags`, `focus_points`, `digest`
- 依赖：`API-02`
- 验收标准：
  所有字段的空值口径一致

### API-11 增加 Phase 1 接口示例响应

- 优先级：`P1`
- 状态：`Done`
- 目标：给前端和测试明确响应样例
- 依赖：`API-05` 到 `API-09`
- 验收标准：
  5 个接口都有可对照样例

### API-12 设计 `market/detail` 与 `insight/*` 的边界

- 优先级：`P1`
- 状态：`Doing`
- 目标：避免新旧接口语义混乱
- 建议：
  `market/detail` 继续偏图表和底层行情
  `insight/*` 专注解释层和内容层
- 依赖：无
- 验收标准：
  开发团队对接口边界没有歧义

### API-13 增加基础查询监控

- 优先级：`P2`
- 状态：`Todo`
- 目标：观察 Phase 1 接口响应时间和查询压力
- 依赖：`API-05` 到 `API-09`
- 验收标准：
  能看出慢查询和热点接口

## 6. 跨线协同任务

### CROSS-01 冻结 Phase 1 字段协议

- 优先级：`P0`
- 状态：`Doing`
- 目标：冻结 5 个接口字段协议
- 参与方：
  产品 / 前端 / 后端
- 依赖：无
- 验收标准：
  接口字段不再频繁变动

### CROSS-02 明确“规则版分数”解释文档

- 优先级：`P0`
- 状态：`Done`
- 目标：把 `sentiment_score`、`attention_score` 的 Phase 1 算法口径文档化
- 参与方：
  后端 / 产品
- 依赖：`BASE-08`, `BASE-09`
- 验收标准：
  分数含义可解释，不是黑盒

### CROSS-03 明确验收样本资产

- 优先级：`P0`
- 状态：`Done`
- 目标：固定一组币种作为联调和验收对象
- 建议：
  `BTC-USDT`, `ETH-USDT`, `SOL-USDT`, `DOGE-USDT`
- 依赖：无
- 验收标准：
  所有联调围绕同一组样本资产进行

### CROSS-04 明确更新时间和缓存策略

- 优先级：`P1`
- 状态：`Todo`
- 目标：定义每个 artifact 的更新频率和 API 读取策略
- 依赖：`BASE-06` 到 `BASE-15`
- 验收标准：
  前后端都知道数据多久更新一次

## 7. 推荐排期切片

## 7.1 下一批优先实现项

基于当前代码进度，下一批最值得继续推进的不是新增更多接口，而是补齐下面这些质量项：

- `CROSS-02`
  把 `sentiment_score`、`attention_score` 的 Phase 1 规则口径文档化，避免前后端和产品解释不一致
- `BASE-12`
  扩展并固定事件类型枚举，补齐 `trend_shift`、`attention_spike`、`price_breakout`
  `risk_warning` 已从单一高分歧触发扩展到真实多来源：
  `defensive_signal`、`attention_divergence`、`elevated_risk`
  当前已验证 API 可返回三类风险事件，前端可以按三种风险卡片同步开发
  `timeline` 已补 `marker_style` 元信息，前端可以直接按 `action | warning | flow | info` 映射 K 线 marker 风格
- `BASE-16`
  收紧 `narrative_tags` 的 Phase 1 策略，减少当前占位标签过粗的问题
- `BASE-10`
  继续提升 `reason_summary`、`summary`、`digest` 的内容质量，降低“系统模板感”
  已补一版 `whale` 内容聚合策略，`timeline / digest` 会优先返回按资产压缩后的摘要流，减少重复事件
  已补一版 `signal` 聚合策略，同一资产同方向在同一小时窗口内会合并成一条时间线/摘要更新
  API 查询层已补“小时窗口 + 重要度”排序，同窗口里 `signal`、`risk_warning` 会优先于 `whale`
- `API-08`
  让 `timeline` 的 `period` 参数真正生效，不再只是保留字段
- `API-12`
  明确 `market/detail` 和 `insight/*` 的边界，方便前端长期接线
- `CROSS-04`
  明确 artifact 更新时间、TTL 和前端读取预期

如果只选一条最短收益路径，建议顺序是：

1. `CROSS-02`
2. `BASE-12`
3. `API-08`
4. `BASE-16`
5. `BASE-10`

### Sprint A

- `DB-01` 到 `DB-09`
- `BASE-01` 到 `BASE-06`
- `BASE-07`
- `BASE-08`
- `BASE-09`
- `CROSS-01`
- `CROSS-03`

目标：

- 把 summary 主链路打通

### Sprint B

- `BASE-10`
- `BASE-11`
- `BASE-12`
- `BASE-13`
- `BASE-14`
- `API-01` 到 `API-08`
- `API-10`
- `CROSS-02`

目标：

- 把 Hero、watchlist、summary、timeline 打通

### Sprint C

- `BASE-15`
- `BASE-16`
- `BASE-17`
- `BASE-18`
- `BASE-20`
- `API-09`
- `API-11`
- `API-12`
- `CROSS-04`

目标：

- 把 digest、降级策略、联调体验补齐

## 8. Phase 1 完成定义

下面这些条件都成立时，backlog 可视为 Phase 1 完成：

1. DB 侧 6 张核心新表已稳定可用。
2. `edgeflow-base` 能按计划产出 5 类 artifact。
3. `edgeflow` 能稳定暴露 5 个 `insight` 接口。
4. 首页 Hero、详情页 summary、timeline、digest 都能接新接口。
5. pipeline run log 可追踪。
6. 新旧 `market/signal` 逻辑未被破坏。
