# Signal / Trend / Decision / Evaluation 四层重构方案

## 1. 目标

这份文档用于把 `edgeflow-base` 当前的信号链路，重新整理成 4 层：

- `Trend`
- `Signal`
- `Decision`
- `Evaluation`

目标不是追求“神准预测”，而是把系统从“会不断被实盘 K 线牵着走的策略流水线”，整理成一套：

- 有稳定背景判断
- 有明确触发时机
- 有可解释策略决策
- 有独立后验评估

这样它才能同时服务：

- App 的信号与图表 marker
- Insight 的 summary / timeline / digest
- 后续的 risk warning / trend shift / setup cluster

## 2. 当前结论

当前系统最大的问题不是“没指标”，而是把 4 件不同的事混在了一条流水线上：

1. 多周期趋势背景判断
2. 15m 触发信号生成
3. 是否落库、是否对外暴露的决策
4. 信号事后盈亏评估

这导致几个直接后果：

- 系统容易被短线 K 线牵着走
- 同一 setup 会被重复生成多条 signal
- 图表 marker 更像“策略日志”，而不是“用户可理解的事件”
- `DecisionTree` 现在更像过滤器，不是真正的策略执行器
- `SignalTracker` 的职责过重，而且现在的抑制没有真正挡住落库

一句话判断：

当前系统更像“多周期技术指标触发器 + 简单过滤器 + 后验回测器”，还不是一套清晰分层的交易/洞察引擎。

## 3. 当前代码映射

### 3.1 K 线更新入口

当前入口：

- [edgeflow-base/cmd/main.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/cmd/main.go:127)
- [edgeflow-base/internal/task/kline.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/task/kline.go:21)
- [edgeflow-base/internal/service/signal/kline/service.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/kline/service.go:1)

当前行为：

- 每 15 分钟拉一次 `15m` K 线
- 在整点或半点补 `30m / 1h / 4h`
- 无差别通知 trend
- trend 再无差别通知 signal

当前问题：

- 传递的是 `struct{}`，没有 symbol/timeframe/close_time 语义
- 整条链路是“全量重跑”，不是“增量触发”
- 后续所有层都不知道“这次到底哪根 bar 真正闭合并变化了”

### 3.2 趋势层

当前实现：

- [edgeflow-base/internal/service/signal/trend/trend.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/trend.go:59)
- [edgeflow-base/internal/service/signal/trend/state_machine.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/state_machine.go:1)

当前职责：

- 从 `30m / 1h / 4h` K 线计算各周期 score
- 聚合成 `TrendScore / FinalScore`
- 用状态机维护 `UP / DOWN / NEUTRAL`

当前问题：

- 趋势层既想“稳”，又想“快”
- `30m` 仍然较强地参与最终分数，容易在边缘区域来回摆动
- 状态机现在承担了过多“预测”期待，但它更适合做降噪和 regime inference
- 趋势状态虽然被保存，但没有被明确建模成“背景层输出”

### 3.3 信号层

当前实现：

- [edgeflow-base/internal/service/signal/indicator/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/signal.go:43)
- [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:96)

当前职责：

- 在 `15m` K 线上计算 EMA / MACD / RSI / ADX / reversal / volume
- 直接输出 `BUY / SELL / REVERSAL_BUY / REVERSAL_SELL`

当前问题：

- 它更像 `trigger engine`，但现在被当成“最终方向判断器”
- 任何连续几根 bar 都可能重复生成同方向信号
- 没有稳定的 setup identity，无法识别“这是老 setup 延续，还是新 setup 诞生”

### 3.4 决策层

当前实现：

- [edgeflow-base/internal/service/signal/decision_tree/decision.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/decision_tree/decision.go:1)
- [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)

当前职责：

- `rawSignal + latestTrendState -> ApplyFilter`
- 决定是否落库

当前问题：

- 现在只是过滤器，不是策略执行器
- 还没有承担：
  - setup 去重
  - cooldown
  - signal 合并
  - “只生成 insight、不生成交易 signal”
  - 风险环境下禁止发信号

### 3.5 评估层

当前实现：

- [edgeflow-base/internal/service/signal/backtest/simulator.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/backtest/simulator.go:123)
- [edgeflow-base/internal/dao/query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:152)

当前职责：

- 维护 `ActiveSignals`
- 用 `SimulateOutcome` 模拟逐根 K 线命中 TP/SL/过期
- 写入 `signal_outcomes`
- 反向给策略提供一定抑制

当前问题：

- 评估层现在半参与主决策，职责过重
- 当前抑制发生在落库之后，挡不住图表 marker 堆叠
- `MaxOverlapCandles` 和 `MaxCandleHoldPeriod` 的职责混用，容易让抑制窗口过短

## 4. 四层目标定义

### 4.1 Trend Layer

目标：

- 负责回答“当前市场背景是什么”
- 不直接产出交易指令

建议输出：

- `regime`
  `bullish | bearish | neutral | transition`
- `confidence`
- `persistence`
- `trend_strength`
- `volatility_regime`
- `trend_bias`

它回答的问题应该是：

- 当前是否适合顺势做多
- 当前是否进入防守/震荡环境
- 当前是否只是过渡区，不适合过早下结论

它不应该回答的问题：

- 现在就该不该开仓
- 下一根 15m K 线会涨还是跌

### 4.2 Signal Layer

目标：

- 负责回答“当前 15m 有没有出现值得关注的触发点”
- 不直接决定是否对外发出 signal

建议输出从“交易指令”改成“触发类型”：

- `trend_follow_breakout`
- `trend_follow_pullback`
- `reversal_attempt`
- `reversal_confirmed`
- `volume_expansion`
- `breakout_retest`

每条触发至少带：

- `setup_type`
- `direction`
- `trigger_bar_time`
- `entry_anchor_price`
- `score`
- `indicator_snapshot`

### 4.3 Decision Layer

目标：

- 真正承担策略执行和产品输出决策

它应该决定：

- 是否生成交易 signal
- 是否只生成 insight event
- 是否合并到已有 setup
- 是否延长已有 setup 的生命周期
- 是否因为风险环境过高而拒绝发 signal

输出可以分成两类：

- `signal_event`
- `insight_event`

这层才是最接近“策略执行器”的位置。

### 4.4 Evaluation Layer

目标：

- 只做后验评价和质量反馈

建议职责：

- 统计 TP / SL / EXPIRED
- 统计不同 setup_type 的命中率、平均持有时长、平均 pnl
- 给 decision layer 提供“最近表现好不好”的背景分
- 为 insight 提供“近几轮信号效果如何”的解释材料

它不应该直接决定：

- 当前是否新建一条 signal

## 5. 保留 / 降级 / 重写建议

## 5.1 Trend 层

### 保留

- 多周期输入框架
  [trend.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/trend.go:116)
- 状态机思路
  [state_machine.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/state_machine.go:1)
- `ScoreForPeriod` 的大部分指标计算
  [trend.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/trend.go:260)

### 降级

- 不再把趋势层视为“预测器”
- 不再要求趋势层直接回答交易时机
- 降低 `30m` 对 regime 切换的影响，把它更多作为确认项

### 重写

- 明确输出 `regime` 结构体，而不是只输出 `TrendState`
- 给状态机加入：
  - 进入阈值和退出阈值分离
  - 最短停留时间
  - `transition` 状态
- 把趋势更新从“全量计算后直接服务 signal”改成“独立背景产物”

## 5.2 Signal 层

### 保留

- 现有技术指标计算组件
  [indicator/indicator.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/indicator.go:1)
- `SignalGenerator` 的指标汇总能力
  [indicator/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/signal.go:17)

### 降级

- 不再让 `signalGen.Generate` 直接代表“最终买卖答案”
- 把 `BUY / SELL / REVERSAL_*` 理解成“候选触发结果”，不是成品 signal

### 重写

- 把输出模型从 `finalAction` 改成：
  - `setup_type`
  - `direction`
  - `trigger_strength`
  - `entry_mode`
- 引入 `setup_key`，例如：
  `symbol + period + direction + setup_type + trigger_bar_time`
- 增加“状态变化才触发”的规则：
  - 首次越过阈值才发
  - setup 延续则更新，不新建

## 5.3 Decision 层

### 保留

- 当前 `DecisionTree` 的过滤入口
  [decision.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/decision_tree/decision.go:1)
- 当前 `processSingleSymbol` 的 orchestration 框架
  [signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:96)

### 降级

- 不再把 `DecisionTree` 只当成 `pass / reject` 过滤器

### 重写

- 升级为真正的 `Policy Engine`

建议最少支持 5 类决策：

- `create_signal`
- `merge_signal`
- `refresh_signal`
- `emit_insight_only`
- `ignore`

建议输入：

- `regime`
- `trigger`
- `active_setup_summary`
- `recent_evaluation_summary`
- `risk_context`

建议输出：

- `decision_type`
- `reason_code`
- `signal_priority`
- `cooldown_window`
- `cluster_key`

## 5.4 Evaluation 层

### 保留

- `SimulateOutcome`
  [simulator.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/backtest/simulator.go:1)
- `signal_outcomes` 表和聚合统计
  [query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:152)

### 降级

- 不再让 tracker 的抑制承担主决策职责
- 不再依赖“内存里是否还有 active signal”来决定是否落库

### 重写

- 明确拆成：
  - `Lifecycle Tracker`
  - `Outcome Evaluator`
  - `Performance Summary`
- 修正当前混用问题：
  - `MaxOverlapCandles` 只负责 cooldown
  - `MaxCandleHoldPeriod` 只负责持有/过期评估
- 评估结果通过 summary 回流到 decision，不直接挡主流程

## 6. 为什么现在容易被实盘 K 线带着走

原因不是单个指标，而是结构问题：

1. `15m` 是最敏感输入，但承担了太大权重
2. 趋势层没有足够强的滞后和持有约束
3. signal 层把连续满足条件的 bar 都当成“新机会”
4. decision 层没有 setup 合并和 cluster 概念
5. evaluation 层的抑制发生在落库之后

所以系统自然会表现成：

- 市场一抖就重算
- 一重算就容易再发 signal
- 一发 signal 图表上就多点

## 7. 面向洞察产品的正确定位

这套系统不应该追求：

- 精确预测下一根 bar 的涨跌

更合理的定位是：

- 用 `Trend` 给出背景
- 用 `Signal` 给出触发
- 用 `Decision` 决定是否值得对外呈现
- 用 `Evaluation` 监控这些呈现是否长期有效

对 Insight 产品来说，最值钱的不是“预测神准”，而是：

- 不乱发
- 有背景
- 有解释
- 有稳定事件结构

## 8. 对 App / Insight 的产物映射

四层重构完成后，建议对外产物分成两类。

### 8.1 交易/图表层

- `signal_clusters`
- `signal_markers`
- `trend_shift_events`
- `risk_warning_events`

图表不要直接消费原始 `signals` 表，而应该消费聚合后的 marker / cluster。

### 8.2 Insight 层

- `asset_insight_snapshots`
- `asset_timeline_events`
- `asset_digest_items`
- `watchlist_candidates`

也就是说：

- `signal` 是内部策略产物
- `insight event` 才是对用户暴露的解释性产物

## 9. 推荐重构顺序

### Phase A

先不改指标，先改职责。

1. 把 `SignalTracker` 从主决策路径里降级
2. 把 `DecisionTree` 升级成 policy engine
3. 给 signal 引入 `setup_key / cluster_key`

### Phase B

再稳定趋势层。

1. 给状态机加 hysteresis
2. 引入 `transition`
3. 输出明确的 `regime` 结构

### Phase C

最后收 signal 对外协议。

1. 图表改吃 `signal_cluster marker`
2. insight 改吃 `signal -> event` 转换结果
3. 老 `signals` 表逐步退回内部评估和调试用途

## 10. 最终建议

当前最不该做的是：

- 继续往 `signalGen.Generate` 里堆更多指标
- 直接引入 LLM 参与价格方向主判断

当前最该做的是：

1. 把趋势、触发、决策、评估彻底拆开
2. 把 `DecisionTree` 从过滤器升级成真正的策略执行层
3. 把图表和 insight 消费对象，从“原始 signal 流”切到“聚合后的事件层”

如果后面要引入 LLM，更适合放在：

- `insight summary`
- `risk explanation`
- `narrative / sentiment / digest`

而不是直接放在价格和开仓方向判断里。
