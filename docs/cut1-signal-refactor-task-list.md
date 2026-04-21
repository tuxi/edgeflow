# Cut 1 开发任务单

## 1. 范围

这份任务单只覆盖 Cut 1：

- `signal_setups`
- `policy engine`
- `signal_clusters`
- `markers API`

目标非常明确：

- 先把重复 signal 砍下来
- 先把图表 marker 从“原始 signal 历史”切到“cluster marker”
- 先让前端拿到更干净的数据集

这份任务单不覆盖：

- `trend_regime_snapshots`
- 状态机重构
- evaluation summary
- timeline 全量切 cluster / event

这些属于下一阶段。

## 2. Cut 1 完成定义

Cut 1 可以认为完成，当满足下面这些条件：

1. `edgeflow-base` 可以落库 `signal_setups`
2. `DecisionTree` 已升级为最小可用 `policy engine`
3. `edgeflow-base` 可以维护 `signal_clusters`
4. `edgeflow` 可以通过新接口返回 cluster marker 数据
5. 前端可以不依赖旧 `signal history` 来画主要 K 线 marker

## 3. 任务拆分

## 3.1 DB 任务

### T1 新增 Cut 1 migration

目标：

- 建立 Cut 1 所需表和兼容字段

建议文件：

- 新增 [edgeflow/script/sql/signal_refactor_cut1.sql](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/script/sql/signal_refactor_cut1.sql:1)

建议内容：

- 新建 `signal_setups`
- 新建 `signal_clusters`
- 新建 `decision_records`
- 为旧 `signals` 增加兼容字段：
  - `setup_key`
  - `cluster_key`
  - `decision_ref_id`

验收标准：

- 本地数据库可执行 migration
- 不影响现有 insight phase1 表

### T2 映射新的 entity

目标：

- 给 Cut 1 新表建立实体结构

建议文件：

- 新增 [edgeflow-base/internal/model/entity/signal_setup.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal_setup.go:1)
- 新增 [edgeflow-base/internal/model/entity/signal_cluster.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal_cluster.go:1)
- 新增 [edgeflow-base/internal/model/entity/decision_record.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/decision_record.go:1)
- 修改 [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:1)

具体改动：

- 新实体字段与 migration 一一对应
- `entity.Signal` 增加：
  - `SetupKey`
  - `ClusterKey`
  - `DecisionRefID`

验收标准：

- GORM 能正确映射
- 编译通过

### T3 扩展 DAO 接口

目标：

- 为 setup / cluster / decision 提供独立 dao

建议文件：

- 修改 [edgeflow-base/internal/dao/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/signal.go:1)
- 修改 [edgeflow-base/internal/service/signal/repository/repo.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/repository/repo.go:1)
- 修改 [edgeflow-base/internal/dao/query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:1)

需要新增的方法：

- `SaveSignalSetup`
- `GetSignalSetupByKey`
- `SaveOrUpdateSignalCluster`
- `GetActiveSignalCluster`
- `CreateDecisionRecord`
- `ListSignalClustersForMarker`

验收标准：

- `edgeflow-base` 内部可独立读写 setup / cluster / decision

## 3.2 edgeflow-base 任务

### T4 定义 Trigger 模型

目标：

- 把 `SignalGenerator` 的输出从“最终动作”改成“候选 trigger”

建议文件：

- 新增 [edgeflow-base/internal/service/signal/model/trigger.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/model/trigger.go:1)
- 修改 [edgeflow-base/internal/service/signal/indicator/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/indicator/signal.go:1)

建议字段：

- `SetupType`
- `Direction`
- `TriggerBarTime`
- `EntryAnchorPrice`
- `TriggerScore`
- `IndicatorSnapshot`
- `SuggestedCommand`

验收标准：

- `signalGen.Generate` 不再只能返回旧 `rawSignal`
- 至少能返回一个中间层 trigger 结构

### T5 实现 `setup_key`

目标：

- 解决同一 setup 的幂等识别问题

建议文件：

- 新增 [edgeflow-base/internal/service/signal/setup/key.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/setup/key.go:1)
- 新增 [edgeflow-base/internal/service/signal/setup/service.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/setup/service.go:1)

建议逻辑：

- 输入：
  `symbol + period + direction + setup_type + trigger_bar_time`
- 输出：
  `setup_key`

验收标准：

- 同一 bar、同一方向、同一 setup 多次运行，`setup_key` 一致

### T6 落库 `signal_setups`

目标：

- 在 `processSingleSymbol` 中，先写 setup，再进入 decision

建议文件：

- 修改 [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:96)

具体改动：

- 保留 `signalTracker.ProcessKlines(...)` 暂时不动
- `signalGen.Generate(...)` 后先生成 trigger
- trigger -> setup
- 先调用 `SaveSignalSetup`
- 再进入 policy engine

验收标准：

- 每轮处理后，`signal_setups` 可写库
- 相同 setup 不重复插入

### T7 新增最小可用 `Policy Engine`

目标：

- 让 `DecisionTree` 升级成真正的执行决策层

建议文件：

- 新增 [edgeflow-base/internal/service/signal/policy/model.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/policy/model.go:1)
- 新增 [edgeflow-base/internal/service/signal/policy/service.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/policy/service.go:1)
- 修改 [edgeflow-base/internal/service/signal/decision_tree/decision.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/decision_tree/decision.go:1)
- 修改 [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)

第一版至少支持：

- `create_signal`
- `merge_signal`
- `refresh_signal`
- `ignore`

第一版最小输入：

- latest setup
- latest active cluster
- latest trend state

第一版最小规则：

- 同 `setup_key` -> `ignore`
- 同方向且在 cooldown 窗口 -> `merge_signal`
- 无活跃 cluster 且通过过滤 -> `create_signal`
- 活跃 cluster 仍在延续 -> `refresh_signal`

验收标准：

- 不再只是 `passed/reject`
- 能产出 decision record

### T8 持久化 `decision_records`

目标：

- 每次决策都能追踪

建议文件：

- 修改 [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)
- 修改 [edgeflow-base/internal/dao/query/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/dao/query/signal.go:1)

具体改动：

- policy engine 返回 `decision_type / reason_code / cluster_key`
- 调用 `CreateDecisionRecord`

验收标准：

- 每次 create/merge/refresh/ignore 都有记录

### T9 新增 Cluster 服务

目标：

- 把对外可展示的 signal 收敛成 `signal_clusters`

建议文件：

- 新增 [edgeflow-base/internal/service/signal/cluster/service.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/cluster/service.go:1)
- 新增 [edgeflow-base/internal/service/signal/cluster/key.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/cluster/key.go:1)
- 修改 [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)

建议逻辑：

- `create_signal` -> 创建 cluster
- `merge_signal` -> 增加 `signal_count`，更新 `last_trigger_time`
- `refresh_signal` -> 更新 cluster 活跃时间和价格锚点

cluster 需要至少输出：

- `cluster_key`
- `cluster_type`
- `direction`
- `first_trigger_time`
- `last_trigger_time`
- `marker_price`
- `signal_count`
- `status`

验收标准：

- `signal_clusters` 可稳定写库
- 同方向连续 signal 能合并到一个 cluster

### T10 保留旧 `signals` 作为兼容产物

目标：

- 不打断旧接口，同时让它和新层有映射关系

建议文件：

- 修改 [edgeflow-base/internal/service/signal/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/service/signal/signal.go:118)
- 修改 [edgeflow-base/internal/model/entity/signal.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/model/entity/signal.go:1)

具体改动：

- 只有 `create_signal` 时才创建新 `signals` 记录
- `merge_signal / refresh_signal` 不再每次新建旧 signal
- 新建旧 signal 时写入：
  - `setup_key`
  - `cluster_key`
  - `decision_ref_id`

验收标准：

- 旧 signal 记录密度明显下降
- 图表历史不再随着每轮 15m 重算持续膨胀

## 3.3 edgeflow 任务

### T11 新增 Marker 查询模型

目标：

- 为新的 cluster marker 建立 API 层模型

建议文件：

- 修改 [edgeflow/internal/model/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/model/insight.go:1)

建议新增结构：

- `AssetMarkerReq`
- `AssetMarkerItem`
- `AssetMarkerRes`

建议字段：

- `cluster_key`
- `marker_time`
- `marker_price`
- `marker_style`
- `marker_priority`
- `cluster_type`
- `direction`
- `signal_count`
- `is_active`

验收标准：

- `edgeflow` 内部有独立 marker 响应模型

### T12 新增 Marker DAO 查询

目标：

- 从 cluster 层读取图表 marker 数据

建议文件：

- 修改 [edgeflow/internal/dao/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/dao/insight.go:1)
- 修改 [edgeflow/internal/dao/query/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/dao/query/insight.go:1)
- 如有必要新增 entity：
  [edgeflow/internal/model/entity/signal_cluster.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/model/entity/signal_cluster.go:1)

建议新增方法：

- `ListAssetMarkers(ctx, instrumentID string, limit int) ([]entity.SignalCluster, error)`

验收标准：

- `edgeflow` 能按资产读取 cluster marker

### T13 新增 Marker Service

目标：

- 在 service 层把 cluster 转成前端可直接消费的 marker

建议文件：

- 修改 [edgeflow/internal/service/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight.go:1)
- 如需内容型映射，可补：
  [edgeflow/internal/service/insight_content.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight_content.go:1)

需要做的事：

- cluster -> marker item
- 映射 `marker_style`
- 映射 `marker_priority`
- 输出 `is_active`

建议映射：

- `trend_follow_* -> action`
- `reversal_* -> warning`
- `volume_expansion -> info`

验收标准：

- service 返回的是干净 marker 集，不是旧 signal 历史

### T14 新增 Markers Handler 和路由

目标：

- 暴露 Cut 1 的新接口

建议文件：

- 修改 [edgeflow/internal/handler/insight/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/handler/insight/insight.go:1)
- 修改 [edgeflow/internal/router/router.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/router/router.go:1)

建议接口：

- `GET /api/v1/insight/assets/:instrument_id/markers`

验收标准：

- 本地可返回真实 marker 数据

### T15 更新 iOS / 前端联调文档

目标：

- 让前端知道从哪个接口拿新 marker，以及怎么和旧 signal history 并行

建议文件：

- 修改 [edgeflow/docs/insight-ios-handoff.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/insight-ios-handoff.md:1)
- 可选新增：
  [edgeflow/docs/cut1-marker-handoff.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/cut1-marker-handoff.md:1)

需要补的内容：

- 新接口说明
- 字段说明
- 新旧接口边界
- 切换建议

验收标准：

- 前端能独立接入 markers API

## 4. 推荐执行顺序

最推荐的实际开发顺序：

1. `T1`
2. `T2`
3. `T3`
4. `T4`
5. `T5`
6. `T6`
7. `T7`
8. `T8`
9. `T9`
10. `T10`
11. `T11`
12. `T12`
13. `T13`
14. `T14`
15. `T15`

## 5. 可并行任务

建议并行方式：

### 并行组 A

- `T1`
- `T2`
- `T3`

这组适合先做基础设施。

### 并行组 B

- `T4`
- `T5`

这组适合先把 signal generator 收敛成 setup 层。

### 并行组 C

- `T11`
- `T12`

这组可以在 `signal_clusters` 表结构定型后先搭 API 骨架。

## 6. 会影响前端联调的节点

真正影响前端的节点只有 3 个：

### 节点 1

`T14` 完成后

含义：

- 前端可以开始接 `markers API`

### 节点 2

`T15` 完成后

含义：

- 前端可以按正式文档切新数据模型

### 节点 3

`T10` 完成并验证后

含义：

- 旧 signal history 的密度会下降
- 前端需要知道旧接口行为和之前不完全一样

## 7. 开发注意事项

Cut 1 开发时，最需要注意的是：

1. 不要一开始就删除旧 `signals`
2. 不要让 `merge_signal / refresh_signal` 继续创建新旧 signal 记录
3. 不要让 `markers API` 直接复用旧 signal history 压缩逻辑
4. `cluster_key` 和 `setup_key` 要从第一天就稳定，否则后面很难迁移

## 8. 下一步

这份任务单完成后，就可以直接进入开发。

最适合的开工点是：

- 先做 `T1 ~ T3`
- 接着做 `T4 ~ T10`
- 最后做 `T11 ~ T15`
