package service

import (
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"fmt"
	"math"
	"strings"
	"time"
)

func localizeHeadlineNarratives(locale string, narratives []model.HeadlineNarrative) []model.HeadlineNarrative {
	items := make([]model.HeadlineNarrative, 0, len(narratives))
	for _, narrative := range narratives {
		narrative.Name = displayNarrativeName(locale, narrative.ID, narrative.Name)
		items = append(items, narrative)
	}
	return items
}

func buildMarketOverviewSummary(locale, sentiment, riskAppetite string, narratives []model.HeadlineNarrative) string {
	topic := describeNarratives(locale, narratives)
	if locale == zhHansLocale {
		switch riskAppetite {
		case "warming":
			return fmt.Sprintf("市场情绪%s，风险偏好%s，资金愿意继续围绕%s寻找扩散机会。", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
		case "cooling":
			return fmt.Sprintf("市场情绪%s，风险偏好%s，当前讨论焦点仍集中在%s，但追价意愿有限。", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
		default:
			return fmt.Sprintf("市场情绪%s，风险偏好%s，当前讨论焦点集中在%s。", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
		}
	}
	switch riskAppetite {
	case "warming":
		return fmt.Sprintf("Market sentiment is %s and risk appetite is %s, with traders still looking for follow-through around %s.", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
	case "cooling":
		return fmt.Sprintf("Market sentiment is %s while risk appetite is %s, so %s remains in focus but follow-through looks selective.", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
	default:
		return fmt.Sprintf("Market sentiment is %s while risk appetite is %s. The main focus is %s.", marketSentimentLabel(locale, sentiment), riskAppetiteLabel(locale, riskAppetite), topic)
	}
}

func buildWatchlistReason(locale, groupType string, snapshot *entity.AssetInsightSnapshot) string {
	if snapshot == nil {
		if locale == zhHansLocale {
			return "该资产进入关注列表，等待更多确认信号。"
		}
		return "This asset entered the watchlist while the system waits for stronger confirmation."
	}

	switch groupType {
	case "sentiment_rising":
		if locale == zhHansLocale {
			return fmt.Sprintf("情绪分升至%.0f，方向性开始增强，适合继续跟踪。", snapshot.SentimentScore)
		}
		return fmt.Sprintf("Sentiment has improved to %.0f, and directional conviction is becoming clearer.", snapshot.SentimentScore)
	case "attention_spike":
		if locale == zhHansLocale {
			return fmt.Sprintf("关注度升至%.0f，市场正在快速聚焦这个资产。", snapshot.AttentionScore)
		}
		return fmt.Sprintf("Attention has climbed to %.0f, signalling that the market is rotating toward this asset.", snapshot.AttentionScore)
	case "divergence":
		if locale == zhHansLocale {
			return fmt.Sprintf("热度与方向判断出现背离，背离分为%.0f，需要谨慎观察。", snapshot.DivergenceScore)
		}
		return fmt.Sprintf("Attention and conviction are diverging, with a divergence score of %.0f.", snapshot.DivergenceScore)
	default:
		if locale == zhHansLocale {
			return "该资产进入关注列表，适合继续观察后续催化。"
		}
		return "This asset entered the watchlist and is worth monitoring for the next catalyst."
	}
}

func buildAssetSummaryText(locale string, snapshot *entity.AssetInsightSnapshot, tags []string) string {
	narrative := primaryNarrativeLabel(locale, tags)
	sentiment := sentimentBandLabel(locale, snapshot.SentimentScore)
	attention := attentionBandLabel(locale, snapshot.AttentionScore)
	riskClause := assetRiskClause(locale, snapshot)
	if locale == zhHansLocale {
		return fmt.Sprintf("%s当前处于%s状态，市场关注度%s，主线叙事偏向%s，%s。", snapshot.Symbol, sentiment, attention, narrative, riskClause)
	}
	return fmt.Sprintf("%s is trading with a %s tone while attention is %s, the dominant narrative is %s, and %s.", snapshot.Symbol, sentiment, attention, narrative, riskClause)
}

func buildFocusPoints(locale string, snapshot *entity.AssetInsightSnapshot, tags []string) []string {
	points := make([]string, 0, 3)
	if snapshot.AttentionChange >= 4 {
		if locale == zhHansLocale {
			points = append(points, "关注度在最近一个周期明显抬升。")
		} else {
			points = append(points, "Attention has accelerated over the latest observation window.")
		}
	}
	if math.Abs(snapshot.SentimentChange) >= 8 {
		if locale == zhHansLocale {
			points = append(points, fmt.Sprintf("情绪变化幅度为%.0f，说明短线方向判断正在重定价。", snapshot.SentimentChange))
		} else {
			points = append(points, fmt.Sprintf("Sentiment moved by %.0f points, suggesting that short-term conviction is being repriced.", snapshot.SentimentChange))
		}
	}
	if snapshot.DivergenceScore >= 18 {
		if locale == zhHansLocale {
			points = append(points, "热度上升快于方向确认，容易出现追涨回落。")
		} else {
			points = append(points, "Attention is outpacing directional confirmation, which raises the chance of a fade after the first move.")
		}
	}
	if len(points) == 0 {
		narrative := primaryNarrativeLabel(locale, tags)
		if locale == zhHansLocale {
			points = append(points, fmt.Sprintf("当前主线仍围绕%s展开，接下来更需要新的催化来放大方向性。", narrative))
		} else {
			points = append(points, fmt.Sprintf("The main setup still revolves around %s, and the next catalyst is needed before conviction can expand.", narrative))
		}
	}
	return points
}

func buildRiskWarning(locale string, snapshot *entity.AssetInsightSnapshot) string {
	switch snapshot.RiskLevel {
	case "elevated":
		if locale == zhHansLocale {
			return "当前风险等级偏高，建议等待确认而不是在噪音里追单。"
		}
		return "Risk is elevated here, so waiting for confirmation is safer than chasing noise."
	case "low":
		if locale == zhHansLocale {
			return "当前波动和分歧都相对可控，但仍需关注下一次情绪切换。"
		}
		return "Volatility and disagreement are relatively contained, but the next sentiment shift still matters."
	default:
		if locale == zhHansLocale {
			return "当前风险中性，适合结合价格与时间线事件一起观察。"
		}
		return "Risk is moderate, so the setup should be read together with price and timeline events."
	}
}

func localizeTimelineItem(locale string, item model.AssetTimelineItem) model.AssetTimelineItem {
	item.Title = timelineTitle(locale, item)
	item.Summary = timelineSummary(locale, item)
	return item
}

func localizeDigestItem(locale string, item model.AssetDigestItem) model.AssetDigestItem {
	item.SourceName = digestSourceDisplay(locale, item.SourceType, item.SourceName)
	item.Title = digestTitle(locale, item)
	item.Summary = digestSummary(locale, item)
	return item
}

func timelineTitle(locale string, item model.AssetTimelineItem) string {
	variant := timelineVariant(item.Timestamp)
	switch item.EventType {
	case "signal_created":
		if locale == zhHansLocale {
			switch variant {
			case 1:
				return "信号继续累积"
			case 2:
				return "短线方向再次确认"
			default:
				return "新的交易信号出现"
			}
		}
		switch variant {
		case 1:
			return "Signal flow keeps building"
		case 2:
			return "Short-term direction firms up again"
		default:
			return "Fresh signal detected"
		}
	case "whale_activity":
		if locale == zhHansLocale {
			switch variant {
			case 1:
				return "主力资金继续停留"
			case 2:
				return "大额账户再次活跃"
			default:
				return "大户活跃度上升"
			}
		}
		switch variant {
		case 1:
			return "Smart-money focus stays in place"
		case 2:
			return "Large accounts show up again"
		default:
			return "Whale activity remains elevated"
		}
	case "risk_warning":
		riskKind := riskWarningKind(item)
		if locale == zhHansLocale {
			switch riskKind {
			case "defensive_signal":
				return "防守信号开始堆积"
			case "attention_divergence":
				return "热度跑在确认前面"
			default:
				return "风险等级抬升"
			}
		}
		switch riskKind {
		case "defensive_signal":
			return "Defensive signal cluster appeared"
		case "attention_divergence":
			return "Attention is outrunning conviction"
		default:
			return "Risk level moved higher"
		}
	default:
		if item.Title != "" {
			return item.Title
		}
		if locale == zhHansLocale {
			return "洞察更新"
		}
		return "Insight update"
	}
}

func timelineSummary(locale string, item model.AssetTimelineItem) string {
	variant := timelineVariant(item.Timestamp)
	switch item.EventType {
	case "signal_created":
		if locale == zhHansLocale {
			if item.SentimentDirection == "positive" {
				switch variant {
				case 1:
					return "偏多信号还在连续出现，说明短线资金没有离开上行延续这条线。"
				case 2:
					return "系统再次给出偏多确认，短线做多情绪仍然在回流。"
				default:
					return "系统识别到新的偏多信号，短线资金开始重新聚焦上行延续。"
				}
			}
			if item.SentimentDirection == "negative" {
				switch variant {
				case 1:
					return "偏空信号继续出现，短线交易仍然更偏向防守和等待确认。"
				case 2:
					return "系统再次给出偏空提示，说明短线资金还没有回到主动进攻。"
				default:
					return "系统识别到新的偏空信号，短线交易更偏向防守和等待确认。"
				}
			}
			return "系统识别到新的方向信号，值得和价格行为一起观察。"
		}
		if item.SentimentDirection == "positive" {
			switch variant {
			case 1:
				return "Bullish signals keep printing, which suggests traders are still leaning into upside continuation."
			case 2:
				return "The engine flagged another bullish confirmation, showing that short-term risk appetite is still rebuilding."
			default:
				return "The signal engine picked up a fresh bullish setup, suggesting that traders are leaning back into upside continuation."
			}
		}
		if item.SentimentDirection == "negative" {
			switch variant {
			case 1:
				return "Bearish signals are still appearing, which keeps short-term positioning tilted toward caution."
			case 2:
				return "The engine flagged another defensive setup, suggesting traders are not ready to re-risk yet."
			default:
				return "The signal engine picked up a fresh bearish setup, nudging short-term positioning back toward caution."
			}
		}
		return "The signal engine detected a fresh setup worth reading alongside price action."
	case "whale_activity":
		if locale == zhHansLocale {
			switch variant {
			case 1:
				return "大额账户还在这个资产附近停留，说明主力资金暂时没有把注意力移开。"
			case 2:
				return "这轮依然能看到大额账户参与，说明它还留在资金观察清单里。"
			default:
				return "大额账户的参与度仍然较高，说明这个资产还在主力资金的观察名单里。"
			}
		}
		switch variant {
		case 1:
			return "Large accounts are still lingering around this asset, which suggests smart-money focus has not faded yet."
		case 2:
			return "Another round of visible large-account participation keeps this asset on the capital watchlist."
		default:
			return "Large-account participation remains visible, which keeps this asset firmly on the smart-money radar."
		}
	case "risk_warning":
		switch riskWarningKind(item) {
		case "defensive_signal":
			if locale == zhHansLocale {
				return "这轮偏空或防守型信号开始累积，说明短线资金还没有回到主动进攻。"
			}
			return "Defensive signal flow is starting to stack up, which suggests short-term traders are not ready to re-risk yet."
		case "attention_divergence":
			if locale == zhHansLocale {
				return "关注度已经跑在方向确认前面，短线更容易出现冲高回落或来回扫动。"
			}
			return "Attention is running ahead of directional confirmation, which raises the chance of fades and whipsaw price action."
		}
		if locale == zhHansLocale {
			return "热度、分歧或风险指标抬升，短线更适合先等确认，而不是在噪音里追价。"
		}
		return "Heat, disagreement, or risk metrics have moved higher, so patience matters more than chasing the first move."
	default:
		if item.Summary != "" {
			return item.Summary
		}
		if locale == zhHansLocale {
			return "系统生成了一条新的洞察更新。"
		}
		return "The system generated a new insight update."
	}
}

func digestTitle(locale string, item model.AssetDigestItem) string {
	if title := timelineTitle(locale, model.AssetTimelineItem{
		EventType:          digestEventType(item.SourceType, item.Title),
		Title:              item.Title,
		SentimentDirection: item.Sentiment,
		Timestamp:          item.PublishedAt,
	}); title != "" {
		return title
	}
	return item.Title
}

func digestSummary(locale string, item model.AssetDigestItem) string {
	return timelineSummary(locale, model.AssetTimelineItem{
		EventType:          digestEventType(item.SourceType, item.Title),
		Title:              item.Title,
		Summary:            item.Summary,
		SentimentDirection: item.Sentiment,
		Timestamp:          item.PublishedAt,
	})
}

func digestEventType(sourceType, title string) string {
	switch sourceType {
	case "signal":
		return "signal_created"
	case "whale":
		return "whale_activity"
	case "system":
		return "risk_warning"
	default:
		lower := strings.ToLower(title)
		switch {
		case strings.Contains(lower, "signal"):
			return "signal_created"
		case strings.Contains(lower, "whale"):
			return "whale_activity"
		case strings.Contains(lower, "risk"):
			return "risk_warning"
		default:
			return ""
		}
	}
}

func digestSourceDisplay(locale, sourceType, fallback string) string {
	switch sourceType {
	case "signal":
		if locale == zhHansLocale {
			return "信号引擎"
		}
		return "Signal Engine"
	case "whale":
		if locale == zhHansLocale {
			return "大户追踪"
		}
		return "Whale Tracker"
	case "system":
		if locale == zhHansLocale {
			return "系统洞察"
		}
		return "System Insight"
	default:
		return fallback
	}
}

func displayNarrativeName(locale, narrativeID, fallback string) string {
	switch narrativeID {
	case "core_asset":
		if locale == zhHansLocale {
			return "核心资产"
		}
		return "Core Assets"
	case "layer2":
		if locale == zhHansLocale {
			return "二层扩容"
		}
		return "Layer 2"
	case "ai":
		if locale == zhHansLocale {
			return "AI 叙事"
		}
		return "AI"
	case "meme":
		if locale == zhHansLocale {
			return "Meme 板块"
		}
		return "Meme"
	case "defi":
		if locale == zhHansLocale {
			return "DeFi"
		}
		return "DeFi"
	default:
		if fallback != "" {
			return fallback
		}
		if locale == zhHansLocale {
			return "市场叙事"
		}
		return "Market Narrative"
	}
}

func describeNarratives(locale string, narratives []model.HeadlineNarrative) string {
	if len(narratives) == 0 {
		if locale == zhHansLocale {
			return "核心资产"
		}
		return "core assets"
	}
	return displayNarrativeName(locale, narratives[0].ID, narratives[0].Name)
}

func primaryNarrativeLabel(locale string, tags []string) string {
	if len(tags) == 0 {
		return displayNarrativeName(locale, "core_asset", "")
	}
	return displayNarrativeName(locale, tags[0], "")
}

func marketSentimentLabel(locale, sentiment string) string {
	switch sentiment {
	case "positive":
		if locale == zhHansLocale {
			return "偏积极"
		}
		return "constructive"
	case "negative":
		if locale == zhHansLocale {
			return "偏谨慎"
		}
		return "fragile"
	default:
		if locale == zhHansLocale {
			return "中性"
		}
		return "balanced"
	}
}

func riskAppetiteLabel(locale, appetite string) string {
	switch appetite {
	case "warming":
		if locale == zhHansLocale {
			return "正在回暖"
		}
		return "warming"
	case "cooling":
		if locale == zhHansLocale {
			return "正在降温"
		}
		return "cooling"
	default:
		if locale == zhHansLocale {
			return "相对稳定"
		}
		return "stable"
	}
}

func assetRiskClause(locale string, snapshot *entity.AssetInsightSnapshot) string {
	if snapshot == nil {
		if locale == zhHansLocale {
			return "目前更适合等下一次催化"
		}
		return "the setup still needs a clearer catalyst"
	}
	if snapshot.DivergenceScore >= 18 {
		if locale == zhHansLocale {
			return "热度已经跑在方向确认前面"
		}
		return "attention is running ahead of conviction"
	}
	switch snapshot.RiskLevel {
	case "elevated":
		if locale == zhHansLocale {
			return "短线波动可能继续放大"
		}
		return "short-term volatility may stay elevated"
	case "low":
		if locale == zhHansLocale {
			return "短线环境相对平稳"
		}
		return "the near-term setup is relatively calm"
	default:
		if locale == zhHansLocale {
			return "下一次催化将决定是否继续扩散"
		}
		return "the next catalyst will decide whether momentum expands"
	}
}

func sentimentBandLabel(locale string, score float64) string {
	switch {
	case score >= 65:
		if locale == zhHansLocale {
			return "偏强"
		}
		return "constructive"
	case score <= 35:
		if locale == zhHansLocale {
			return "偏弱"
		}
		return "cautious"
	default:
		if locale == zhHansLocale {
			return "中性"
		}
		return "balanced"
	}
}

func attentionBandLabel(locale string, score float64) string {
	switch {
	case score >= 70:
		if locale == zhHansLocale {
			return "明显升温"
		}
		return "running hot"
	case score <= 35:
		if locale == zhHansLocale {
			return "相对平淡"
		}
		return "still muted"
	default:
		if locale == zhHansLocale {
			return "温和活跃"
		}
		return "moderately active"
	}
}

func timelineVariant(ts time.Time) int {
	if ts.IsZero() {
		return 0
	}
	return int(ts.Unix()/3600) % 3
}

func riskWarningKind(item model.AssetTimelineItem) string {
	lower := strings.ToLower(item.EventID + " " + item.Title + " " + item.Summary)
	switch {
	case strings.Contains(lower, "defensive signal"):
		return "defensive_signal"
	case strings.Contains(lower, "attention_divergence"), strings.Contains(lower, "outrunning conviction"):
		return "attention_divergence"
	default:
		return "elevated_risk"
	}
}
