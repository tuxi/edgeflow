# Signal / Trend / Decision / Evaluation 四层重构 Backlog

## 1. 说明

这份 backlog 直接承接 [signal-trend-decision-evaluation-refactor.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/signal-trend-decision-evaluation-refactor.md:1)，把四层重构拆成三条主线：

- `DB / 表结构`
- `edgeflow-base`
- `edgeflow`

目标不是一次性推翻旧链路，而是：

- 先把职责拆清楚
- 再把信号从“原始流水”收敛成“稳定 setup / cluster / event”
- 最后让 App 和 Insight 改吃更适合产品表达的产物

任务拆分原则：

- 优先做“能明显减少 signal 重复和 marker 堆叠”的项
- 保持新旧链路可并行，避免一次性切换风险
- 先做 `edgeflow-base` 内部重构，再收敛 `edgeflow` 输出协议

优先级定义：

- `P0`：主链路阻塞项，建议优先排
- `P1`：重要项，建议本期完成
- `P2`：增强项，可后置

状态建议：

- `Todo`
- `Doing`
- `Done`
- `Blocked`

## 2. 开发总顺序

推荐顺序：

1. 先做 `DB / 表结构`
2. 再做 `edgeflow-base` 的四层拆分
3. 最后接 `edgeflow` 的查询和前端协议

原因：

- 没有新的 setup / cluster / event 落库目标，`edgeflow-base` 不好拆
- 没有稳定产物，`edgeflow` 继续直接吃旧 `signals` 只会把旧问题延续到前端

## 3. DB / 表结构 Backlog

### DB-01 新增 `trend_regime_snapshots` 表

- 优先级：`P0`
- 状态：`Todo`
- 目标：把趋势层输出显式沉淀为“背景层产物”
- 主要字段：
  `snapshot_id`, `instrument_id`, `symbol`, `regime`, `trend_bias`, `confidence`, `persistence`, `trend_strength`, `volatility_regime`, `score_30m`, `score_1h`, `score_4h`, `final_score`, `snapshot_time`
- 依赖：无
- 验收标准：
  可按 `instrument_id + snapshot_time` 查询最新背景状态

### DB-02 新增 `signal_setups` 表

- 优先级：`P0`
- 状态：`Todo`
- 目标：存储 signal 层生成的“候选 setup”，不再直接等同于最终 signal
- 主要字段：
  `setup_id`, `setup_key`, `instrument_id`, `symbol`, `period`, `setup_type`, `direction`, `trigger_bar_time`, `entry_anchor_price`, `trigger_score`, `indicator_snapshot_json`, `status`, `created_at`, `updated_at`
- 依赖：无
- 验收标准：
  同一 setup 可以被幂等识别，不会每轮重跑都插一条新记录

### DB-03 新增 `signal_clusters` 表

- 优先级：`P0`
- 状态：`Todo`
- 目标：沉淀对外可展示的 signal cluster，而不是直接暴露原始 signal 流
- 主要字段：
  `cluster_id`, `cluster_key`, `instrument_id`, `symbol`, `direction`, `cluster_type`, `first_trigger_time`, `last_trigger_time`, `entry_anchor_price`, `marker_price`, `signal_count`, `priority`, `status`, `latest_setup_ref_id`, `created_at`, `updated_at`
- 依赖：`DB-02`
- 验收标准：
  同一段连续 setup 能合并成一个 cluster

### DB-04 新增 `decision_records` 表

- 优先级：`P1`
- 状态：`Todo`
- 目标：记录 decision layer 的决策结果，方便追查为什么发/不发
- 主要字段：
  `decision_id`, `instrument_id`, `symbol`, `setup_ref_id`, `regime_ref_id`, `decision_type`, `reason_code`, `cooldown_until`, `cluster_key`, `risk_gate`, `payload_json`, `created_at`
- 依赖：`DB-01`, `DB-02`
- 验收标准：
  每条对外 signal 或 insight event 都能追溯到决策记录

### DB-05 新增 `evaluation_summaries` 表

- 优先级：`P1`
- 状态：`Todo`
- 目标：把 evaluation 层结果沉淀成 setup_type 级别的汇总，供 decision 读取
- 主要字段：
  `summary_id`, `symbol`, `setup_type`, `direction`, `window`, `sample_count`, `hit_tp_count`, `hit_sl_count`, `expired_count`, `avg_pnl_pct`, `avg_hold_bars`, `win_rate`, `updated_at`
- 依赖：现有 `signal_outcomes`
- 验收标准：
  decision layer 可读取最近窗口的策略表现

### DB-06 为 `signals` 表补“兼容迁移字段”策略

- 优先级：`P0`
- 状态：`Todo`
- 目标：明确旧 `signals` 在过渡期内如何继续使用
- 建议方案：
  保留旧表
  新增可选字段：
  `setup_key`, `cluster_key`, `decision_ref_id`, `regime_ref_id`
- 依赖：`DB-02`, `DB-03`, `DB-04`
- 验收标准：
  旧接口还能跑，新链路也能逐步接入

### DB-07 输出四层重构 migration 脚本

- 优先级：`P0`
- 状态：`Todo`
- 目标：形成可执行 migration，支持本地/测试环境一致建表
- 依赖：`DB-01` 到 `DB-06`
- 验收标准：
  新表和兼容字段能一键创建

## 4. edgeflow-base Backlog

### BASE-01 新增 `trend` 领域输出模型

- 优先级：`P0`
- 状态：`Todo`
- 目标：把当前 `TrendState` 收敛成更稳定的 `RegimeSnapshot`
- 依赖：`DB-01`
- 验收标准：
  趋势层输出显式包含：
  `regime / confidence / persistence / trend_strength / volatility_regime`

### BASE-02 重构 trend 状态机，加入 `transition`

- 优先级：`P0`
- 状态：`Todo`
- 目标：让趋势层更像 regime inference，不是即时预测器
- 依赖：`BASE-01`
- 验收标准：
  状态机支持：
  `bullish / bearish / neutral / transition`

### BASE-03 给 trend 状态机加入 hysteresis 和最短停留时间

- 优先级：`P0`
- 状态：`Todo`
- 目标：减少边缘区间来回跳变
- 依赖：`BASE-02`
- 验收标准：
  进入阈值和退出阈值分离
  状态切换不是每根 bar 都能发生

### BASE-04 把 trend 结果落库到 `trend_regime_snapshots`

- 优先级：`P0`
- 状态：`Todo`
- 目标：让趋势层产物独立存在，而不是只存在内存和 signal 调用链里
- 依赖：`DB-01`, `BASE-03`
- 验收标准：
  每轮趋势更新后可查询到最新 regime snapshot

### BASE-05 把 `SignalGenerator` 输出从“最终动作”改成“候选 trigger”

- 优先级：`P0`
- 状态：`Todo`
- 目标：把 signal 层从最终买卖判断降级成 trigger engine
- 依赖：无
- 验收标准：
  输出最少包含：
  `setup_type`, `direction`, `trigger_bar_time`, `trigger_score`, `entry_anchor_price`

### BASE-06 设计 `setup_key` 生成规则

- 优先级：`P0`
- 状态：`Todo`
- 目标：解决同一 setup 重复生成问题
- 建议输入：
  `symbol + period + direction + setup_type + trigger_bar_time`
- 依赖：`BASE-05`, `DB-02`
- 验收标准：
  同一 setup 在同一 bar 上重复跑不会重复创建

### BASE-07 落库 `signal_setups`

- 优先级：`P0`
- 状态：`Todo`
- 目标：把 raw signal 改存成 setup 层产物
- 依赖：`DB-02`, `BASE-06`
- 验收标准：
  `processSingleSymbol` 可以先写 setup，再进入 decision

### BASE-08 升级 `DecisionTree` 为 `Policy Engine`

- 优先级：`P0`
- 状态：`Todo`
- 目标：让 decision layer 真正承担执行决策
- 依赖：`DB-04`, `BASE-04`, `BASE-07`
- 验收标准：
  最少支持 5 类输出：
  `create_signal`, `merge_signal`, `refresh_signal`, `emit_insight_only`, `ignore`

### BASE-09 给 policy engine 加入 cooldown / dedupe / merge 规则

- 优先级：`P0`
- 状态：`Todo`
- 目标：减少 signal 重复和 marker 堆叠
- 依赖：`BASE-08`
- 验收标准：
  同方向、同 setup、同窗口内不会重复生成一串 signal

### BASE-10 生成并维护 `signal_clusters`

- 优先级：`P0`
- 状态：`Todo`
- 目标：把对外可展示的信号收敛成 cluster 层
- 依赖：`DB-03`, `BASE-08`, `BASE-09`
- 验收标准：
  对外图表层可以不再直接依赖原始 `signals`

### BASE-11 调整旧 `signals` 生成逻辑为“兼容产物”

- 优先级：`P1`
- 状态：`Todo`
- 目标：旧表继续保留，但降级成内部兼容层
- 依赖：`DB-06`, `BASE-10`
- 验收标准：
  旧 `signals` 仍可读，但来源于 cluster / decision 结果，而不是原始连续触发流

### BASE-12 把 `SignalTracker` 从主决策路径里降级

- 优先级：`P0`
- 状态：`Todo`
- 目标：评估层不再阻塞 signal 主流程
- 依赖：无
- 验收标准：
  tracker 只做 lifecycle / evaluation，不再负责是否允许落库

### BASE-13 修正 `MaxOverlapCandles` 和 `MaxCandleHoldPeriod` 的职责

- 优先级：`P0`
- 状态：`Todo`
- 目标：拆开 cooldown 和持有评估
- 依赖：`BASE-12`
- 验收标准：
  cooldown 和 expiry 分别使用独立参数，不再混用

### BASE-14 生成 `evaluation_summaries`

- 优先级：`P1`
- 状态：`Todo`
- 目标：把 evaluation 结果做成策略表现汇总
- 依赖：`DB-05`, `BASE-12`, `BASE-13`
- 验收标准：
  decision layer 可读取不同 `setup_type` 的近期胜率和平均表现

### BASE-15 把 K 线更新通道从 `struct{}` 升级成事件结构

- 优先级：`P1`
- 状态：`Todo`
- 目标：从“全量重跑”向“增量触发”过渡
- 建议字段：
  `symbol`, `timeframe`, `bar_close_time`, `update_type`
- 依赖：无
- 验收标准：
  trend / signal 能知道是哪类 bar 更新，不再只收到空通知

### BASE-16 把 trend 和 signal 监听从“全量跑所有 symbol”改成“可增量”

- 优先级：`P1`
- 状态：`Todo`
- 目标：减少重复计算和连续重复产出
- 依赖：`BASE-15`
- 验收标准：
  至少支持“按 symbol / timeframe 定向重算”

### BASE-17 新增 `signal_event` 到 `insight event` 的映射层

- 优先级：`P1`
- 状态：`Todo`
- 目标：让 insight timeline 吃的是解释性事件，不是原始 signal 流
- 依赖：`BASE-10`
- 验收标准：
  `signal cluster` 可稳定映射成 timeline / digest 所需事件

### BASE-18 回填四层设计文档和规则文档

- 优先级：`P1`
- 状态：`Todo`
- 目标：让策略和产品对同一套口径达成一致
- 依赖：`BASE-04` 到 `BASE-17`
- 验收标准：
  至少补齐：
  regime 规则、setup_key 规则、decision reason_code、evaluation 指标口径

## 5. edgeflow Backlog

### API-01 新增面向图表的 `signal cluster marker` 查询模型

- 优先级：`P0`
- 状态：`Todo`
- 目标：让图表层不再直接消费旧 `signals`
- 依赖：`DB-03`, `BASE-10`
- 验收标准：
  `edgeflow` 内部已有 `cluster marker` model / dao / service

### API-02 新增 `cluster marker` 查询接口

- 优先级：`P0`
- 状态：`Todo`
- 目标：给详情页 K 线提供更干净的 marker 数据集
- 建议接口：
  `GET /api/v1/insight/assets/{instrument_id}/markers`
- 依赖：`API-01`
- 验收标准：
  前端可直接用 cluster marker 画图，不需要自己压缩 signal 历史

### API-03 让 `timeline` 改读 cluster / event 层，而不是原始 signal 流

- 优先级：`P0`
- 状态：`Todo`
- 目标：统一图表事件和时间线事件的来源
- 依赖：`BASE-17`
- 验收标准：
  `timeline` 里的 signal 相关事件来自 cluster/event 层

### API-04 为 marker 返回稳定的展示元信息

- 优先级：`P0`
- 状态：`Todo`
- 目标：让前端不再猜 marker 展示逻辑
- 建议字段：
  `marker_style`, `marker_priority`, `cluster_type`, `direction`, `is_active`
- 依赖：`API-02`
- 验收标准：
  iOS/前端能只按协议渲染，不做二次推导

### API-05 让详情页 signal 相关接口逐步退到兼容层

- 优先级：`P1`
- 状态：`Todo`
- 目标：保留旧接口，但不再鼓励页面继续直接接原始 signal 列表
- 依赖：`API-02`, `API-03`
- 验收标准：
  新页面走 `markers / timeline / digest`
  旧 signal 接口只保兼容用途

### API-06 为 `summary / digest` 注入 regime 背景文案

- 优先级：`P1`
- 状态：`Todo`
- 目标：让趋势层真正服务 insight 解释
- 依赖：`DB-01`, `BASE-04`
- 验收标准：
  `summary / digest` 能引用：
  `bullish / bearish / neutral / transition`

### API-07 为前端补 `decision reason` 和 `signal confidence` 透出字段

- 优先级：`P1`
- 状态：`Todo`
- 目标：增强可解释性，减少“为什么这条 signal 出现”不透明的问题
- 依赖：`DB-04`, `BASE-08`
- 验收标准：
  关键页面能拿到：
  `reason_code`, `confidence`, `cluster_status`

### API-08 更新 iOS 对接文档和前端联调文档

- 优先级：`P0`
- 状态：`Todo`
- 目标：让前端切换到新的 cluster / marker / timeline 协议
- 依赖：`API-02`, `API-03`, `API-04`
- 验收标准：
  文档清楚说明：
  新旧接口边界、字段映射、降级策略、联调顺序

## 6. 推荐排期切片

### Sprint A

先把“重复 signal / marker 堆叠”打下来。

- `DB-02`
- `DB-03`
- `DB-04`
- `BASE-05`
- `BASE-06`
- `BASE-07`
- `BASE-08`
- `BASE-09`
- `BASE-10`
- `API-01`
- `API-02`

### Sprint B

再把 trend 变成真正的 regime 层。

- `DB-01`
- `BASE-01`
- `BASE-02`
- `BASE-03`
- `BASE-04`
- `API-06`

### Sprint C

最后稳定 evaluation 和兼容切换。

- `DB-05`
- `DB-06`
- `BASE-11`
- `BASE-12`
- `BASE-13`
- `BASE-14`
- `API-03`
- `API-04`
- `API-05`
- `API-07`
- `API-08`

## 7. 最短收益路径

如果只想先解决“信号太多、图表 marker 堆叠、洞察不稳定”这三个问题，最短路径建议是：

1. 做 `setup_key`
2. 做 `policy engine`
3. 做 `signal cluster`
4. 让图表和 timeline 改吃 `cluster / event`

也就是说，最先不一定要重写所有指标，而是先把：

- 重复生成
- 重复落库
- 重复展示

这三层问题切断。

## 8. 完成定义

四层重构的第一阶段可以认为完成，当满足下面这些条件：

1. 趋势层已经能稳定输出 `regime snapshot`，而不是只存在内存里。
2. 信号层已经有 `setup_key`，不会再每轮都生成重复 setup。
3. decision layer 已经能输出 `create / merge / refresh / emit_insight_only / ignore`。
4. 评估层已经从主决策里降级，只负责后验和 summary。
5. 图表和时间线已经不再直接消费原始 `signals`，而是消费 `cluster / event`。
6. 前端联调文档已经切到新协议，旧 signal 接口只留兼容用途。
