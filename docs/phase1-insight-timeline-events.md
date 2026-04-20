# Phase 1 Insight Timeline 事件类型说明

这份文档用于定义 `timeline` / `marker` 的事件类型口径，并明确：

- 当前代码已经实际产出的事件
- Phase 1 计划内应该统一支持的事件类型
- 需求文档里提过、但当前还没进入 Phase 1 主线的事件

目标是让前端、后端、产品对 `event_type` 有同一套理解。

当前代码依据：

- [edgeflow-base/internal/insight/builder/builder.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/insight/builder/builder.go:1)
- [edgeflow/internal/model/insight.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/model/insight.go:63)
- [edgeflow/internal/service/insight_content.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/internal/service/insight_content.go:129)

相关需求文档依据：

- [phase1-insight-implementation-checklist.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/phase1-insight-implementation-checklist.md:218)
- [insight-tool-backend-requirements.md](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow/docs/insight-tool-backend-requirements.md:378)

## 1. 当前结论

当前 `GET /api/v1/insight/assets/{instrument_id}/timeline` 已经能稳定返回事件，但真实产出的 `event_type` 只有 3 类：

- `signal_created`
- `whale_activity`
- `risk_warning`

也就是说：

- timeline 链路已可用
- 事件协议已可用
- 但 Phase 1 目标枚举还没有完全产出

当前还没真正产出的 Phase 1 事件：

- `trend_shift`
- `attention_spike`
- `price_breakout`

需求文档里还提过、但不建议现在就混进 Phase 1 的事件：

- `sentiment_shift`
- `narrative_switch`
- `news_event`

## 2. Timeline 字段口径

当前时间线 item 结构：

- `event_id`
- `event_type`
- `title`
- `summary`
- `timestamp`
- `impact_level`
- `sentiment_direction`
- `marker_price`
- `related_narrative`
- `related_source_type`

这些字段已经对前端开放，建议前端继续按这个模型接，不要再做另一套事件结构。

## 3. Phase 1 建议统一的事件类型枚举

当前建议把 Phase 1 正式口径收敛到 6 个值：

- `signal_created`
- `trend_shift`
- `attention_spike`
- `price_breakout`
- `whale_activity`
- `risk_warning`

这 6 个值里：

- 前 3 个和 `signal / trend / price` 强相关
- `whale_activity` 来自资金侧
- `risk_warning` 来自系统综合判断

## 4. 当前已实际产出的事件

## 4.1 `signal_created`

状态：

- `已产出`

当前来源：

- active signal list

当前触发逻辑：

- 每个 active signal 都会产出一条 `signal_created`

当前字段特征：

- `related_source_type = signal`
- `source_ref_type = signal`
- `source_ref_id = signal_id`
- `marker_price = signal.MarkPrice`

当前语义：

- 系统识别到一个新的方向性信号
- 更接近“信号出现 / 继续有效”而不是“状态变更历史”

当前标题/摘要：

- 中文：`新的交易信号出现`
- 英文：`Fresh signal detected`

当前 `sentiment_direction`：

- `BUY` -> `positive`
- `SELL` -> `negative`
- 其他 -> `neutral`

## 4.2 `whale_activity`

状态：

- `已产出`

当前来源：

- HyperLiquid whale leaderboard

当前触发逻辑：

- 当前 top whale 数据会生成 whale 事件

当前字段特征：

- `related_source_type = whale`
- `source_ref_type = whale`
- `source_ref_id = whale.Address`
- `marker_price = null`

当前语义：

- 大额账户活跃或参与度较高
- 属于“资金关注仍在”的提示事件

当前标题/摘要：

- 中文：`大户活跃度上升`
- 英文：`Whale activity remains elevated`

当前 `sentiment_direction`：

- `PnLDay > 0` 或 `ROIDay > 0` -> `positive`
- `PnLDay < 0` 或 `ROIDay < 0` -> `negative`
- 否则 -> `neutral`

注意：

当前 `dominantSymbolFromWhale()` 还是 Phase 1 的简化映射，不代表已经有完整的“鲸鱼 -> 资产”归因能力。

## 4.3 `risk_warning`

状态：

- `已产出`

当前来源：

- `asset_insight_snapshots`

当前触发逻辑：

- `risk_level == elevated`
- 或 `divergence_score >= 18`

满足任一条件时，会生成一条 `risk_warning`

当前字段特征：

- `related_source_type = system`
- `source_ref_type = snapshot`
- `source_ref_id = snapshot.ID`
- `marker_price = null`

当前语义：

- 系统认为当前资产的风险、波动或背离程度上升

当前标题/摘要：

- 中文：`风险等级抬升`
- 英文：`Risk level moved higher`

当前 `sentiment_direction`：

- 固定为 `neutral`

## 5. Phase 1 已定义但尚未产出的事件

下面这些事件已经在文档里被定义成 Phase 1 目标，但当前 builder 还没真正生成。

## 5.1 `trend_shift`

状态：

- `未产出`

建议来源：

- `trend_snapshots`
- 或 signal/trend 状态切换结果

建议语义：

- 趋势方向从多转空、空转多，或从趋势明确切回中性

前端可理解为：

- 比 `signal_created` 更偏“趋势变化”
- 更适合做 K 线上的阶段性标记

当前缺口：

- builder 还没有读取 trend snapshot 变化
- 也没有“上一个状态 vs 当前状态”的对比逻辑

## 5.2 `attention_spike`

状态：

- `未产出`

建议来源：

- `attention_score` / `attention_change`
- 事件数量跃升
- 真实 mention 数据接入后也可复用

建议语义：

- 关注度短时间快速抬升

前端可理解为：

- 这类事件适合和 Hero/Watchlist 的热度升温形成联动

当前缺口：

- `attention_score` 已经有
- 但还没有单独的 spike 阈值和事件落库逻辑

## 5.3 `price_breakout`

状态：

- `未产出`

建议来源：

- 短周期价格异常波动
- 关键阈值突破
- 结合现有 K 线或 ticker 数据

建议语义：

- 价格行为出现明显突破/跌破

前端可理解为：

- 最适合做 `marker_price` 的价格型事件

当前缺口：

- 现在 builder 没有用价格窗口或 breakout 条件去产出事件

## 6. 需求文档中提过、但不进入当前 Phase 1 主线的事件

## 6.1 `sentiment_shift`

状态：

- `需求文档提过`
- `当前不建议并入 Phase 1 正式枚举`

原因：

- 当前 `sentiment_score` 还是规则版
- 没有足够稳定的“前后窗口情绪变化”逻辑

更适合后续在情绪算法稳定后引入。

## 6.2 `narrative_switch`

状态：

- `需求文档提过`
- `当前不建议并入 Phase 1 正式枚举`

原因：

- `narrative_tags` 仍然是占位策略
- 没有稳定的 narrative graph 或 narrative strength

## 6.3 `news_event`

状态：

- `需求文档提过`
- `当前不建议并入 Phase 1 正式枚举`

原因：

- 还没有真实新闻/X 内容源接入主链路

## 7. 前端当前应该怎么接

## 7.1 可以直接按已产出事件先做 UI

当前前端可以稳定接这 3 类：

- `signal_created`
- `whale_activity`
- `risk_warning`

建议：

- 每类做独立 icon / 颜色 / 文案映射
- `marker_price != null` 时允许挂到 K 线 marker

## 7.2 要对未知类型做兼容

虽然当前只有 3 类，但前端不要写死成只能处理 3 个值。

建议：

- 未知 `event_type` 走默认样式
- 仍展示 `title`
- 仍展示 `summary`

这样后端后续补 `trend_shift`、`attention_spike`、`price_breakout` 时，不会卡前端发版。

## 7.3 当前最适合的枚举映射

建议 iOS/前端本地映射按这个口径先做：

- `signal_created`: 信号 / 趋势方向事件
- `whale_activity`: 资金异动事件
- `risk_warning`: 风险提示事件
- `trend_shift`: 趋势切换事件
- `attention_spike`: 热度异动事件
- `price_breakout`: 价格突破事件

## 8. 当前代码与文档差异总结

当前差异可以直接总结成一句话：

- `timeline` 协议已经完成
- `timeline` 枚举目标已经明确
- 但当前真实产物只覆盖了 6 个目标事件里的 3 个

具体对应如下：

| 事件类型 | 文档目标 | 当前代码产出 | 备注 |
| --- | --- | --- | --- |
| `signal_created` | 是 | 是 | 已可用 |
| `trend_shift` | 是 | 否 | 还没接 trend 变化 |
| `attention_spike` | 是 | 否 | 还没做 spike 判定 |
| `price_breakout` | 是 | 否 | 还没接价格突破逻辑 |
| `whale_activity` | 是 | 是 | 已可用 |
| `risk_warning` | 是 | 是 | 已可用 |
| `sentiment_shift` | 否（需求文档提过） | 否 | 后续再看 |
| `narrative_switch` | 否（需求文档提过） | 否 | 后续再看 |
| `news_event` | 否（需求文档提过） | 否 | 等内容源 |

## 9. 下一步建议

如果继续推进 BASE-12，最合理的顺序是：

1. 先把正式 Phase 1 枚举冻结为 6 个值
2. 先补 `trend_shift`
3. 再补 `attention_spike`
4. 再补 `price_breakout`

这样好处是：

- 不会一下子把事件系统做得太散
- 能优先覆盖最贴近当前数据链路的事件
- 前端已经接好的 timeline 结构不用改
