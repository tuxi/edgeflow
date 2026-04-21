# 四层重构实施顺序建议

## 1. 目标

这份文档是在 [signal-trend-decision-evaluation-refactor.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/signal-trend-decision-evaluation-refactor.md:1) 和 [signal-trend-decision-evaluation-backlog.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/signal-trend-decision-evaluation-backlog.md:1) 的基础上，再往下压一层：

- 明确先改哪些文件
- 明确哪些任务可以并行
- 明确哪些节点会影响前端联调

目标不是一次性重写，而是：

- 先切掉重复 signal 和 marker 堆叠
- 再稳定 trend / decision / evaluation 的职责边界
- 最后再推动前端从旧 signal 接口迁到 cluster / marker / event

## 2. 总体实施原则

整个实施顺序建议遵守 3 个原则：

1. 先加新层，不先删旧层
2. 先做兼容写入，再做读取切换
3. 先改 `edgeflow-base` 产物，再改 `edgeflow` 协议

也就是说：

- 旧 `signals` 先保留
- 新的 `signal_setups / signal_clusters / trend_regime_snapshots` 先并行产出
- `edgeflow` 先加新查询接口和新 model
- 前端确认稳定后，再逐步把页面切到新链路

## 3. 第一阶段：先解决 signal 重复和 marker 堆叠

这是最优先阶段，也是最短收益路径。

### 3.1 先改哪些文件

#### DB

- [edgeflow/script/sql/insight_phase1.sql](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/script/sql/insight_phase1.sql:1)
  当前可继续扩展，或新增专门的 `signal_refactor_phase1.sql`

#### edgeflow-base

- [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:1)
- [edgeflow-base/internal/dao/query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:1)
- [edgeflow-base/internal/service/signal/indicator/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/signal.go:1)
- [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:1)
- 新增建议目录：
  - `edgeflow-base/internal/service/signal/setup`
  - `edgeflow-base/internal/service/signal/policy`
  - `edgeflow-base/internal/service/signal/cluster`

#### edgeflow

- [edgeflow/internal/model/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/model/insight.go:1)
- [edgeflow/internal/dao/query/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/dao/query/insight.go:1)
- [edgeflow/internal/service/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight.go:1)
- [edgeflow/internal/handler/insight/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/handler/insight/insight.go:1)

### 3.2 第一阶段建议实施顺序

#### Step A1

先做新表和兼容字段。

任务：

- `DB-02 signal_setups`
- `DB-03 signal_clusters`
- `DB-04 decision_records`
- `DB-06 signals 兼容字段`
- `DB-07 migration`

原因：

- 没有 setup / cluster / decision 落库目标，后面 service 很难改
- 先把兼容字段加好，旧链路还能继续跑

#### Step A2

把 `SignalGenerator` 输出改成 trigger，不再直接作为最终 signal。

任务：

- `BASE-05`
- `BASE-06`
- `BASE-07`

建议改动位置：

- [indicator/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/signal.go:1)
- [signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:96)

目标：

- 先写 `signal_setups`
- 暂时还保留旧 `signals`

#### Step A3

把 `DecisionTree` 升级成 `Policy Engine`。

任务：

- `BASE-08`
- `BASE-09`

建议改动位置：

- [decision_tree/decision.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/decision_tree/decision.go:1)
- [signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)

目标：

- policy engine 不再只返回 `pass/reject`
- 至少支持：
  - `create_signal`
  - `merge_signal`
  - `refresh_signal`
  - `emit_insight_only`
  - `ignore`

#### Step A4

生成 `signal_clusters`。

任务：

- `BASE-10`
- `BASE-11`

建议改动位置：

- 新增 `cluster` 服务
- [query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:1)

目标：

- 旧 `signals` 继续保留
- 但对外新增 cluster 层

#### Step A5

`edgeflow` 新增 `cluster marker` 查询接口。

任务：

- `API-01`
- `API-02`
- `API-04`

目标：

- 前端拿到新的 marker 数据集
- 不再直接压缩旧 signal 历史

### 3.3 哪些任务可以并行

这一阶段推荐并行方式：

- A1 的 DB 任务可单独并行
- A2 和 A3 可部分并行
  但 `BASE-08` 需要等待 `setup_key` 和 `signal_setups` 基本定型
- A5 的 `edgeflow` 查询层可以在 A4 后半段并行

最合理的并行拆法：

1. 一个人先做 migration 和 entity/dao
2. 一个人改 `signalGen -> setup`
3. 一个人开始搭 `edgeflow` 的新接口骨架

### 3.4 这一阶段会不会影响前端

结论：

- `A1 ~ A4` 基本不影响前端
- `A5` 开始影响前端，但属于“新增接口，不破坏旧页面”

所以前端联调的第一个节点不是一开始，而是：

- `API-02 markers` 能返回真实数据之后

届时前端可以先并行做：

- 新 K 线 marker 数据模型
- 新旧 marker 切换开关

## 4. 第二阶段：把 trend 稳定成 regime 层

第一阶段解决的是“重复和堆叠”，第二阶段解决的是“背景不稳、被 K 线带着走”。

### 4.1 先改哪些文件

- [edgeflow-base/internal/service/signal/trend/trend.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/trend.go:1)
- [edgeflow-base/internal/service/signal/trend/state_machine.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/trend/state_machine.go:1)
- [edgeflow-base/internal/service/signal/model/trend.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/model/trend.go:1)
- [edgeflow-base/internal/dao/query/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/insight.go:1)

### 4.2 第二阶段建议实施顺序

#### Step B1

先定义新的 `RegimeSnapshot` 输出模型和表。

任务：

- `DB-01`
- `BASE-01`

#### Step B2

重构状态机，加入：

- `transition`
- hysteresis
- 最短停留时间

任务：

- `BASE-02`
- `BASE-03`

#### Step B3

把趋势结果写入 `trend_regime_snapshots`，并让 decision 读取它，而不是只读内存态趋势。

任务：

- `BASE-04`

#### Step B4

让 `edgeflow` 的 `summary / digest / timeline` 开始引用 regime 文案。

任务：

- `API-06`

### 4.3 哪些任务可以并行

- `B1` 和 `B2` 可以部分并行
- `B4` 必须等 `B3` 有真实产物后再接

### 4.4 这一阶段会不会影响前端

会，但不是协议破坏，而是内容变化。

影响点：

- summary 文案会更稳定
- digest / timeline 的解释会更像“背景 + 触发”而不是单条 signal 描述

前端同步节点：

- 当 `API-06` 落地后，可以通知前端：
  summary / digest 的趋势解释口径已更新

## 5. 第三阶段：把 evaluation 从主决策中拆出来

这是为了让系统更像“可持续优化的策略引擎”，而不是“边评估边乱挡主流程”。

### 5.1 先改哪些文件

- [edgeflow-base/internal/service/signal/backtest/simulator.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/backtest/simulator.go:1)
- [edgeflow-base/internal/dao/query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:152)
- [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:100)

### 5.2 第三阶段建议实施顺序

#### Step C1

先把 tracker 从主决策路径里降级。

任务：

- `BASE-12`

目标：

- 先决策，后评估
- 不再“先落库后被 tracker 逻辑半拒绝”

#### Step C2

修正 cooldown 和 expiry 参数职责。

任务：

- `BASE-13`

目标：

- `MaxOverlapCandles` 只负责 cooldown
- `MaxCandleHoldPeriod` 只负责持有期和过期评估

#### Step C3

生成 evaluation summary。

任务：

- `DB-05`
- `BASE-14`

目标：

- 给 decision layer 一个“最近这种 setup 好不好”的读取入口

### 5.3 哪些任务可以并行

- `C1` 和 `C2` 建议同一个人连续做
- `C3` 可以在 `C2` 收尾时并行开始

### 5.4 这一阶段会不会影响前端

基本不直接影响协议，但会影响：

- signal 数量
- signal 稳定性
- timeline 里 signal event 的频率

这意味着前端不一定要同步开发，但需要知道：

- 数据密度会进一步下降
- event 的“持续时间”会更合理

## 6. 第四阶段：把前端正式切到新协议

这阶段不是“开始做前端”，而是“可以稳定切前端”。

### 6.1 切前端前的最低完成条件

至少要满足：

1. `signal_clusters` 已经稳定写库
2. `markers` 接口已经可用
3. `timeline` 已经读 cluster / event 层
4. `marker_style / marker_priority / cluster_type` 已明确
5. 新旧接口边界文档已更新

### 6.2 先改哪些文件

- [edgeflow/internal/model/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/model/insight.go:1)
- [edgeflow/internal/service/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight.go:1)
- [edgeflow/docs/insight-ios-handoff.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/insight-ios-handoff.md:1)

### 6.3 第四阶段建议实施顺序

#### Step D1

先让 timeline 改读 cluster / event。

任务：

- `API-03`

#### Step D2

再把旧 signal 相关接口降级成兼容层。

任务：

- `API-05`

#### Step D3

最后统一更新联调文档。

任务：

- `API-07`
- `API-08`

### 6.4 这一阶段会不会影响前端

会，而且是明确的联调节点。

前端建议同步顺序：

1. 先接 `markers`
2. 再接新的 `timeline`
3. 再减少对旧 signal 历史接口的依赖

## 7. 并行建议

如果按团队协作排，最推荐的并行方式是：

### 方案 A

三人并行

- 任务线 1：DB + entity + dao
- 任务线 2：edgeflow-base 的 setup / policy / cluster
- 任务线 3：edgeflow 的 query / handler / docs

### 方案 B

两人并行

- 人员 1：
  `DB + edgeflow-base`
- 人员 2：
  `edgeflow + 文档 + 前端联调支持`

## 8. 最短可上线切片

如果你想尽快看到效果，而不是等整个四层重构做完，最短切片建议是：

### Cut 1

只做：

- `signal_setups`
- `policy engine`
- `signal_clusters`
- `markers API`

这一步完成后，就能明显改善：

- marker 堆叠
- signal 重复
- 图表可读性

### Cut 2

再做：

- `trend_regime_snapshots`
- 新状态机
- summary / digest regime 文案

这一步完成后，洞察质量会更稳。

### Cut 3

最后做：

- evaluation summary
- timeline 全量切 cluster / event
- 前端正式迁到新协议

## 9. 建议的实际开发顺序

如果只给一条最推荐顺序，我建议就是：

1. `DB-02`
2. `DB-03`
3. `DB-04`
4. `DB-06`
5. `BASE-05`
6. `BASE-06`
7. `BASE-07`
8. `BASE-08`
9. `BASE-09`
10. `BASE-10`
11. `API-01`
12. `API-02`
13. `DB-01`
14. `BASE-01`
15. `BASE-02`
16. `BASE-03`
17. `BASE-04`
18. `API-06`
19. `BASE-12`
20. `BASE-13`
21. `DB-05`
22. `BASE-14`
23. `API-03`
24. `API-04`
25. `API-05`
26. `API-07`
27. `API-08`

## 10. 最终建议

整个实施过程中，最需要避免的是：

- 一开始就重写所有指标
- 一开始就强行让前端切全部新协议
- 一开始就删除旧 `signals`

最稳的推进方式是：

- 先产出新层
- 再并行验证
- 再逐步切读路径
- 最后再考虑淘汰旧链路
