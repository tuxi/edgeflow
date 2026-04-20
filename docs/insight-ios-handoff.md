# Insight 接口前端对接说明

这份文档给 iOS 端做第一轮接入使用。

目标只有一个：

- 让首页 Hero、详情页 Insight Summary、详情页 Timeline、详情页 Digest 可以开始接真实接口

当前可对接接口：

- `GET /api/v1/insight/market/overview`
- `GET /api/v1/insight/market/watchlist`
- `GET /api/v1/insight/assets/{instrument_id}/summary`
- `GET /api/v1/insight/assets/{instrument_id}/timeline`
- `GET /api/v1/insight/assets/{instrument_id}/digest`

## 1. 当前状态

这 5 个接口已经完成：

- 后端真实查库
- `edgeflow-base` 定时产出 artifact
- `edgeflow` 返回真实数据
- `locale` 已接通
- `en` / `zh-Hans` 内容文案已开始生效

当前适合前端先接的页面模块：

- 首页顶部 Hero
- 币种详情页 Insight 摘要
- 币种详情页 Timeline
- 币种详情页 Digest

## 2. Locale 规则

### 2.1 请求方式

后端支持两种 locale 传递方式：

- Header: `T-Language-Id`
- Query: `?locale=...`

建议前端继续默认走 Header：

- `T-Language-Id: en`
- `T-Language-Id: zh-Hans`

调试时也可以加 query：

- `?locale=en`
- `?locale=zh-CN`
- `?locale=zh-Hans`

### 2.2 规范化规则

后端会 normalize 到：

- `en`
- `zh-Hans`

例如：

- `en-US` -> `en`
- `en-SG` -> `en`
- `zh-CN` -> `zh-Hans`
- `zh` -> `zh-Hans`

### 2.3 前端如何处理

- 固定 UI 文案仍由前端本地化
- 接口里的动态内容文本直接展示，不要二次翻译
- 枚举字段不要直接展示 raw value，前端自己本地化映射

例如这些字段属于枚举：

- `sentiment`
- `risk_appetite`
- `risk_level`
- `group_type`
- `event_type`
- `impact_level`
- `sentiment_direction`
- `source_type`
- `narrative_tags`

## 3. 页面与接口映射

### 3.1 首页 Hero

接口：

- `GET /api/v1/insight/market/overview`

推荐使用字段：

- `sentiment`
- `risk_appetite`
- `headline_narratives`
- `summary`
- `leader_assets`
- `updated_at`

### 3.2 市场页 Watchlist

接口：

- `GET /api/v1/insight/market/watchlist`

推荐使用字段：

- `instrument_id`
- `symbol`
- `group_type`
- `rank_score`
- `reason_summary`
- `sentiment_score`
- `sentiment_change`
- `attention_score`
- `attention_change`
- `narrative_tags`
- `risk_level`
- `divergence_score`
- `updated_at`

### 3.3 详情页 Insight Summary

接口：

- `GET /api/v1/insight/assets/{instrument_id}/summary`

推荐使用字段：

- `instrument_id`
- `symbol`
- `sentiment_score`
- `attention_score`
- `mention_count`
- `mention_change_pct`
- `narrative_tags`
- `summary`
- `focus_points`
- `risk_level`
- `risk_warning`
- `updated_at`

### 3.4 详情页 Timeline

接口：

- `GET /api/v1/insight/assets/{instrument_id}/timeline`

推荐使用字段：

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

当前行为补充：

- `whale_activity` 事件现在会在后端按资产聚合，不再按单个 whale 地址逐条返回
- 同一资产的 whale 更新会按小时窗口压缩，前端不需要再额外做地址级去重
- `signal_created` 事件也会按资产、方向和小时窗口压缩
- 同一小时内连续出现的同方向 signal，前端通常只会看到一条合并后的 signal 更新
- `timeline` 会按“小时窗口 + 重要度”排序；同一窗口里通常 `signal`、`risk_warning` 会排在 `whale_activity` 前面

### 3.5 详情页 Digest

接口：

- `GET /api/v1/insight/assets/{instrument_id}/digest`

推荐使用字段：

- `digest_id`
- `source_type`
- `source_name`
- `title`
- `summary`
- `sentiment`
- `related_narratives`
- `published_at`
- `source_url`

当前行为补充：

- `digest` 中的 `whale` 项也已经改成资产级摘要流
- 首屏如果看到 `whale` 项，通常代表“这一小时内该资产有一组大户活跃”，不是单个地址明细
- `digest` 中的 `signal` 项也会按小时窗口做去重
- 同方向、同一小时的 signal 不会在 digest 里重复刷多条
- `digest` 也会按“小时窗口 + 重要度”排序；同一窗口里 `signal` 一般会比 `whale` 更靠前

## 4. 接口说明

## 4.1 Market Overview

请求：

```http
GET /api/v1/insight/market/overview
T-Language-Id: zh-Hans
```

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "sentiment": "neutral",
    "risk_appetite": "cooling",
    "headline_narratives": [
      {
        "id": "core_asset",
        "name": "核心资产",
        "heat_score": 3180,
        "sentiment": "neutral",
        "assets": ["BTC", "ETH"]
      }
    ],
    "summary": "市场情绪中性，风险偏好正在降温，当前讨论焦点集中在核心资产。",
    "leader_assets": ["BTC", "ETH", "OKB", "SOL", "DOGE"],
    "updated_at": "2026-04-19T01:15:30+08:00"
  }
}
```

字段说明：

- `sentiment`: 市场整体情绪枚举，前端映射展示
- `risk_appetite`: 市场风险偏好枚举，前端映射展示
- `headline_narratives[].name`: 已按 locale 本地化，可直接展示
- `summary`: 动态内容文案，可直接展示
- `leader_assets`: 可用于头像组、标签组、scroll chips

## 4.2 Market Watchlist

请求：

```http
GET /api/v1/insight/market/watchlist?limit=3&locale=zh-CN
```

可选 query：

- `group`: `sentiment_rising | attention_spike | divergence`
- `limit`: 默认 20
- `locale`

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "instrument_id": "AAVE-USDT",
      "symbol": "AAVE",
      "group_type": "sentiment_rising",
      "rank_score": 78,
      "reason_summary": "情绪分升至68，方向性开始增强，适合继续跟踪。",
      "sentiment_score": 68,
      "sentiment_change": 10,
      "attention_score": 40,
      "attention_change": 4,
      "narrative_tags": ["core_asset"],
      "risk_level": "moderate",
      "divergence_score": 0,
      "updated_at": "2026-04-19T01:15:30+08:00"
    }
  ]
}
```

字段说明：

- `group_type`: 当前分组来源
- `reason_summary`: 当前已按 locale 生成，可直接展示
- `rank_score`: 用于排序，不建议直接展示给用户
- `narrative_tags`: 语义 tag，建议前端映射展示名称

## 4.3 Asset Summary

请求：

```http
GET /api/v1/insight/assets/BTC-USDT/summary?locale=zh-CN
```

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "instrument_id": "BTC-USDT",
    "symbol": "BTC",
    "sentiment_score": 32,
    "attention_score": 40,
    "mention_count": null,
    "mention_change_pct": null,
    "narrative_tags": ["core_asset"],
    "summary": "BTC当前处于偏弱状态，市场关注度温和活跃，主线叙事偏向核心资产。",
    "focus_points": [
      "关注度在最近一个周期明显抬升。",
      "情绪变化幅度为-10，说明短线方向判断正在重定价。"
    ],
    "risk_level": "moderate",
    "risk_warning": "当前风险中性，适合结合价格与时间线事件一起观察。",
    "updated_at": "2026-04-19T01:15:30+08:00"
  }
}
```

字段说明：

- `summary`: 详情页 Insight 摘要主文案
- `focus_points`: 建议最多展示前 2 到 3 条
- `mention_count` / `mention_change_pct`: Phase 1 允许为 `null`
- `risk_warning`: 可作为卡片底部风险提示

## 4.4 Asset Timeline

请求：

```http
GET /api/v1/insight/assets/BTC-USDT/timeline?locale=zh-CN
```

可选 query：

- `limit`: 默认 20
- `period`: 当前支持 `24h | 7d`，默认 `7d`
- `locale`

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "event_id": "whale_0xecb63caa47c7c4e77f60f1ce858cf28dc2b82b00_5921775",
      "event_type": "whale_activity",
      "title": "大户活跃度上升",
      "summary": "大额账户的参与度仍然较高，说明这个资产还在资金关注范围内。",
      "timestamp": "2026-04-19T01:15:30+08:00",
      "impact_level": "medium",
      "sentiment_direction": "positive",
      "marker_price": null,
      "related_narrative": "core_asset",
      "related_source_type": "whale"
    },
    {
      "event_id": "signal_1378",
      "event_type": "signal_created",
      "title": "新的交易信号出现",
      "summary": "系统识别到偏空信号，短线情绪转向谨慎。",
      "timestamp": "2026-04-19T01:00:00+08:00",
      "impact_level": "medium",
      "sentiment_direction": "negative",
      "marker_price": 75793.1,
      "related_narrative": "core_asset",
      "related_source_type": "signal"
    }
  ]
}
```

字段说明：

- `event_id`: 列表 key
- `event_type`: 事件类型枚举
- `title` / `summary`: 已按 locale 生成，可直接展示
- `marker_price`: 可选；若前端要和 K 线 marker 联动，可复用
- `related_source_type`: 当前常见值为 `signal | whale | system`
- `period`: 当前只有 `24h` 和 `7d` 两档；传非法值会返回校验错误

## 4.5 Asset Digest

请求：

```http
GET /api/v1/insight/assets/BTC-USDT/digest?locale=en
```

可选 query：

- `limit`: 默认 20
- `locale`

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "digest_id": "digest_signal_1378",
      "source_type": "signal",
      "source_name": "Signal Engine",
      "title": "Fresh signal detected",
      "summary": "The signal engine picked up a bearish setup, which is pushing the market toward caution.",
      "sentiment": "negative",
      "related_narratives": ["core_asset"],
      "published_at": "2026-04-19T01:00:00+08:00",
      "source_url": ""
    }
  ]
}
```

字段说明：

- `source_name`: 已按 locale 本地化，可直接展示
- `title` / `summary`: 已按 locale 生成，可直接展示
- `source_url`: Phase 1 可能为空字符串

## 5. iOS 侧建议的数据处理

## 5.1 建议直接展示的字段

这些字段当前可以直接给用户看：

- `summary`
- `reason_summary`
- `title`
- `source_name`
- `risk_warning`
- `focus_points`
- `headline_narratives[].name`

## 5.2 建议前端本地化映射的字段

这些字段建议前端自己做枚举显示：

- `sentiment`
- `risk_appetite`
- `risk_level`
- `group_type`
- `event_type`
- `impact_level`
- `sentiment_direction`
- `source_type`
- `narrative_tags`

当前 `narrative_tags` 白名单值：

- `core_asset`
- `layer2`
- `ai`
- `meme`
- `defi`

## 5.3 建议先隐藏或降级的字段

Phase 1 内这些字段先不要强依赖：

- `mention_count`
- `mention_change_pct`
- `source_url`

处理建议：

- `null` 或空字符串时直接隐藏
- 不要为了占位强行显示 `--`

## 6. 当前已知限制

这是 Phase 1 的真实状态，前端接入时请按这个预期来：

- `timeline` 和 `digest` 目前主要来自 `signal` 和 `whale` 的系统生成内容
- `watchlist` 是规则排序，不是最终推荐系统
- `narrative_tags` 还是偏规则化的语义标签
- `mention_count` / `mention_change_pct` 还没有接真实社媒数据
- 内容质量已经可用，但还不是最终版本

换句话说：

- 协议和接入方式现在可以稳定
- 内容质量后面还会继续迭代

## 7. 联调顺序建议

建议 iOS 先按这个顺序同步：

1. 首页 Hero 接 `market/overview`
2. 详情页 Summary 接 `assets/{instrument_id}/summary`
3. 详情页 Timeline 接 `assets/{instrument_id}/timeline`
4. 详情页 Digest 接 `assets/{instrument_id}/digest`
5. 市场页 Watchlist 再接 `market/watchlist`

这样可以先完成最核心的产品观感。

## 8. 错误与空态建议

### 8.1 接口错误

统一按现有公共网络层处理。

### 8.2 空数据

如果接口 `data` 为空或数组为空，建议：

- Hero: 隐藏 insight 文案区域
- Summary: 显示固定空态文案
- Timeline: 显示 “No insight events yet”
- Digest: 显示 “No digest yet”

### 8.3 时间格式

后端返回的是标准时间字符串，前端按本地语言格式化展示即可。

## 9. 当前建议的前端模型

前端可以先直接按接口字段建模，不建议现在做二次抽象合并。

原因：

- 当前还在 Phase 1
- 字段仍会继续丰富
- 先保持与接口 1:1 更利于联调

等页面稳定后，再考虑 ViewModel 层统一。
