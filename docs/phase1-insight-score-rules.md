# Phase 1 Insight 分数解释文档

这份文档用于对齐当前 Phase 1 里 `sentiment_score`、`attention_score`、`divergence_score`、`risk_level` 的真实口径。

目标不是定义理想算法，而是解释“当前代码正在怎么算”，方便：

- 前端展示分数时有统一说法
- 产品理解分数含义
- 后续迭代时能明确哪些地方是 Phase 1 占位规则

当前代码依据：

- [edgeflow-base/internal/insight/builder/builder.go](/Users/xiaoyuan/Documents/work/git/edgeflow-server/edgeflow-base/internal/insight/builder/builder.go:1)

## 1. 当前原则

Phase 1 的分数规则有 4 个特点：

- 规则型，不是模型型
- 只依赖当前已有数据源
- 优先可解释，不追求高精度
- 用于“排序、分组、解释”，不用于策略执行

当前主要输入源只有：

- active instruments
- latest price
- active signals
- signal summary
- HyperLiquid whale leaderboard

暂时还没有直接接入：

- 新闻
- X / 社媒提及量
- 完整 narrative 图谱
- LLM 级内容理解

所以前端在展示时，建议把这些分数理解为：

- `sentiment_score`: 当前方向判断偏强还是偏弱
- `attention_score`: 当前市场关注度高不高
- `divergence_score`: 热度和方向判断是否出现背离

而不是把它理解成“预测涨跌概率”。

## 2. 字段总览

当前 Phase 1 与分数相关的核心字段：

- `sentiment_score`
- `sentiment_change`
- `attention_score`
- `attention_change`
- `divergence_score`
- `risk_level`
- `reason_summary`
- `risk_warning`

这些字段主要出现在：

- `asset_insight_snapshots`
- `watchlist_candidates`
- `market_overview_snapshots`

## 3. `sentiment_score`

## 3.1 分数范围

- 范围：`0 ~ 100`
- 默认基线：`50`

前端建议理解：

- `0 ~ 34`: 偏弱 / cautious
- `35 ~ 64`: 中性 / balanced
- `65 ~ 100`: 偏强 / constructive

这不是后端硬编码枚举，而是当前展示建议。

## 3.2 当前输入项

Phase 1 里，`sentiment_score` 主要只看 active signal：

- `BUY`
- `SELL`
- 无 signal

## 3.3 当前计算规则

起点：

- 初始值 = `50`

如果有 active signal：

- `BUY` -> `sentiment_score += 18`
- `SELL` -> `sentiment_score -= 18`

如果没有 active signal：

- 保持在基线附近
- 同时生成解释文案：
  `no fresh signal, sentiment driven by baseline market state`

最终结果会被 clamp 到：

- 最小 `0`
- 最大 `100`

### 公式表达

```text
sentiment_score = clamp(50 + signal_bias, 0, 100)

signal_bias:
BUY  = +18
SELL = -18
NONE = 0
```

## 3.4 当前局限

当前 `sentiment_score` 还没有真正纳入：

- trend final score
- price direction
- narrative strength
- 社媒情绪

所以它目前本质上更接近：

- “方向性偏向分”

而不是完整的“市场情绪分”。

## 4. `sentiment_change`

## 4.1 分数范围

当前不是百分比，也不是相对前值差分，而是规则型变化量。

常见值：

- `+10`
- `0`
- `-10`

## 4.2 当前计算规则

起点：

- 初始值 = `0`

如果有 active signal：

- `BUY` -> `sentiment_change += 10`
- `SELL` -> `sentiment_change -= 10`

如果没有 active signal：

- 保持 `0`

### 公式表达

```text
sentiment_change:
BUY  = +10
SELL = -10
NONE = 0
```

## 4.3 前端展示建议

建议把它理解成：

- “本周期方向变化强度”

不建议把它展示成：

- “24h 真正变化百分比”

如果前端要展示箭头或趋势标签，可以用：

- `> 0`: 升温
- `= 0`: 稳定
- `< 0`: 走弱

## 5. `attention_score`

## 5.1 分数范围

- 范围：`0 ~ 100`
- 默认基线：`35`

前端建议理解：

- `0 ~ 34`: 关注度低
- `35 ~ 59`: 温和活跃
- `60 ~ 100`: 关注度明显升高

## 5.2 当前输入项

Phase 1 当前使用的输入：

- latest price 是否存在
- signal 历史 closed count
- signal summary 的 win rate
- signal 是否 premium
- signal 是否足够新鲜

## 5.3 当前计算规则

起点：

- 初始值 = `35`

### A. 价格已跟踪

如果当前资产有最新价格：

- `attention_score += 8`

这意味着：

- 只要进入正常行情跟踪，注意力分就会从 `35` 提到 `43`

### B. signal 历史闭环数

如果 signal summary 存在：

- `attention_score += min(total_closed_signals, 12)`

解释：

- closed signals 越多，说明系统对这个资产有更多可回看的记录
- 这一项最多加 `12`

### C. premium signal

如果当前 signal 是 premium：

- `attention_score += 6`

### D. 新鲜 signal

如果 signal 时间距离当前小于 `6 小时`：

- `attention_score += 5`

### 公式表达

```text
attention_score =
  clamp(
    35
    + price_bonus
    + closed_signal_bonus
    + premium_bonus
    + freshness_bonus,
    0,
    100
  )

price_bonus:
tracked price exists = +8

closed_signal_bonus:
min(total_closed_signals, 12)

premium_bonus:
premium signal = +6

freshness_bonus:
signal within 6h = +5
```

## 5.4 当前局限

当前 `attention_score` 还没有真正接入文档中原计划的输入：

- 波动率代理
- 事件数量
- whale activity 强度
- mention count / mention change
- 榜单变化

所以它现在更接近：

- “系统内部关注强度分”

而不是完整的“市场热度分”。

## 6. `attention_change`

## 6.1 当前输入项

当前只看两项：

- signal summary 的 win rate
- signal 是否在最近 `6 小时`

## 6.2 当前计算规则

起点：

- 初始值 = `0`

如果 signal summary 存在：

- `attention_change += min(win_rate / 10, 8)`

如果 signal 时间小于 `6 小时`：

- `attention_change += 4`

### 公式表达

```text
attention_change =
  min(win_rate / 10, 8)
  + freshness_delta

freshness_delta:
signal within 6h = +4
otherwise         = 0
```

### 举例

- `win_rate = 63%`
  -> `63 / 10 = 6.3`
  -> 加 `6.3`
- 如果同时 signal 在 6 小时内
  -> 再加 `4`
  -> 最终 `attention_change = 10.3`

## 6.3 前端展示建议

建议把它理解成：

- “当前关注度变化速度”

不建议把它直接命名成：

- “24h 热度变化百分比”

## 7. `divergence_score`

## 7.1 含义

这个分数用来描述：

- 关注度已经升高
- 但方向判断还没有同步变强或变弱

也就是文档里说的：

- 热度和方向判断背离

## 7.2 当前触发条件

只有在下面两个条件同时满足时才会开始计算：

- `attention_score >= 60`
- `sentiment_score < 45` 或 `sentiment_score > 55`

这说明系统只会在“已经比较热”且“方向偏离基线”时才看背离。

## 7.3 当前计算公式

```text
divergence_score = abs(attention_score - sentiment_score) * 0.6
```

最终结果也会 clamp 到：

- `0 ~ 100`

## 7.4 当前解释阈值

如果：

- `divergence_score > 18`

builder 会额外补一条原因：

- `attention is moving faster than directional conviction`

前端建议理解：

- `0 ~ 17`: 背离不明显
- `18+`: 背离开始值得提示

## 8. `risk_level`

## 8.1 当前可选值

- `low`
- `moderate`
- `elevated`

## 8.2 当前规则

默认：

- `risk_level = moderate`

如果满足任一条件：

- `attention_score >= 70`
- `divergence_score >= 18`

则：

- `risk_level = elevated`

如果同时满足：

- `attention_score < 40`
- `sentiment_score` 在 `45 ~ 55` 之间

则：

- `risk_level = low`

### 伪代码

```text
risk_level = moderate

if attention_score >= 70 or divergence_score >= 18:
  risk_level = elevated

if attention_score < 40 and 45 <= sentiment_score <= 55:
  risk_level = low
```

## 8.3 前端展示建议

- `low`: 环境平稳，但也代表催化不足
- `moderate`: 当前默认状态
- `elevated`: 热度高或背离明显，应该提醒风险

注意：

`elevated` 不等于 bearish。

它表达的是：

- 短线波动和不确定性上升

## 9. `reason_summary` 和 `risk_warning`

这两个字段不是分数，但和分数解释强相关。

## 9.1 `reason_summary`

来源：

- 根据当前规则往 `reasons` 数组里写模板句子
- 最后最多取前两条并拼接

可能出现的原因包括：

- `active buy signal supports positive bias`
- `active sell signal keeps sentiment cautious`
- `premium signal currently active`
- `attention is moving faster than directional conviction`
- `no fresh signal, sentiment driven by baseline market state`

也就是说：

- 它不是自由生成内容
- 是分数规则的解释摘要

## 9.2 `risk_warning`

当前只有 3 档模板：

- `elevated` 或 `divergence_score >= 18`
  -> `Short-term volatility may expand if follow-through weakens.`
- `low`
  -> `Current setup looks stable, but momentum is still limited.`
- 其他
  -> `Watch for a clearer catalyst before expanding conviction.`

## 10. Watchlist 排序口径

前端在 `market/watchlist` 上看到的排序，其实直接依赖分数。

当前 3 个分组规则：

### 10.1 `sentiment_rising`

排序分：

```text
rank_score = sentiment_score + sentiment_change
```

### 10.2 `attention_spike`

排序分：

```text
rank_score = attention_score + attention_change
```

### 10.3 `divergence`

排序分：

```text
rank_score = divergence_score
```

每个分组：

- 按 `rank_score` 倒序
- 只保留前 `20` 条

## 11. Market Overview 口径

首页 Hero 里的市场级结论，是资产级分数再聚合出来的。

## 11.1 `market_sentiment`

用全量 snapshot 的平均 `sentiment_score`：

```text
avg_sentiment = sum(sentiment_score) / asset_count
```

规则：

- `avg_sentiment >= 58` -> `positive`
- `avg_sentiment <= 42` -> `negative`
- 否则 -> `neutral`

## 11.2 `risk_appetite`

用全量 snapshot 的平均 `attention_score`：

```text
avg_attention = sum(attention_score) / asset_count
```

规则：

- `avg_attention >= 60` -> `warming`
- `avg_attention <= 40` -> `cooling`
- 否则 -> `stable`

## 12. 前端展示建议

为了避免误导用户，当前版本建议：

### 12.1 可以直接展示数值

- `sentiment_score`
- `attention_score`
- `divergence_score`

### 12.2 最好同时给一层文案标签

例如：

- `sentiment_score`
  - `0 ~ 34`: 偏弱
  - `35 ~ 64`: 中性
  - `65 ~ 100`: 偏强
- `attention_score`
  - `0 ~ 34`: 关注度低
  - `35 ~ 59`: 温和活跃
  - `60 ~ 100`: 关注度升温
- `divergence_score`
  - `0 ~ 17`: 背离不明显
  - `18+`: 背离明显

### 12.3 不建议前端自己推导

前端不要自己根据多个字段再拼一个新的“情绪标签”或“热度等级”，否则：

- 首页和详情页可能解释不一致
- watchlist 排序口径和详情页展示口径会冲突

## 13. 当前版本最重要的限制

前端和产品需要一起接受这个现实：

1. 这是规则版分数，不是模型版分数。
2. `sentiment_score` 当前过度依赖 active signal。
3. `attention_score` 当前还不是完整热度，只是系统关注强度代理。
4. `mention_count` / `mention_change_pct` 目前仍是占位。
5. narrative 和内容质量还在 Phase 1 迭代中。

所以当前最好的使用方式是：

- 作为排序、提示、解释辅助
- 不作为单独的交易判断依据

## 14. 下一步建议

如果继续迭代这套分数规则，建议优先补：

1. 把 trend score 纳入 `sentiment_score`
2. 把 price move / event count / whale strength 纳入 `attention_score`
3. 让 `mention_count` / `mention_change_pct` 接真实社媒数据
4. 把 narrative strength 做成独立字段
5. 把分数变化量改成真实时间窗口差分，而不是纯规则增量
