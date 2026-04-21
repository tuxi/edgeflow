package model

import "time"

type MarketOverviewRes struct {
	Sentiment          string              `json:"sentiment"`
	RiskAppetite       string              `json:"risk_appetite"`
	HeadlineNarratives []HeadlineNarrative `json:"headline_narratives"`
	Summary            string              `json:"summary"`
	LeaderAssets       []string            `json:"leader_assets"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

type HeadlineNarrative struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	HeatScore float64  `json:"heat_score"`
	Sentiment string   `json:"sentiment"`
	Assets    []string `json:"assets"`
}

type MarketWatchlistReq struct {
	Group  string `form:"group" json:"group"`
	Limit  int    `form:"limit" json:"limit"`
	Locale string `form:"locale" json:"locale"`
}

type MarketWatchlistItem struct {
	InstrumentID    string    `json:"instrument_id"`
	Symbol          string    `json:"symbol"`
	GroupType       string    `json:"group_type"`
	RankScore       float64   `json:"rank_score"`
	ReasonSummary   string    `json:"reason_summary"`
	SentimentScore  float64   `json:"sentiment_score"`
	SentimentChange float64   `json:"sentiment_change"`
	AttentionScore  float64   `json:"attention_score"`
	AttentionChange float64   `json:"attention_change"`
	NarrativeTags   []string  `json:"narrative_tags"`
	RiskLevel       string    `json:"risk_level"`
	DivergenceScore float64   `json:"divergence_score"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AssetInsightSummaryReq struct {
	InstrumentID string `form:"instrument_id" json:"instrument_id" uri:"instrument_id" binding:"required"`
}

type AssetInsightSummaryRes struct {
	InstrumentID     string    `json:"instrument_id"`
	Symbol           string    `json:"symbol"`
	SentimentScore   float64   `json:"sentiment_score"`
	AttentionScore   float64   `json:"attention_score"`
	MentionCount     *int64    `json:"mention_count"`
	MentionChangePct *float64  `json:"mention_change_pct"`
	NarrativeTags    []string  `json:"narrative_tags"`
	Summary          string    `json:"summary"`
	FocusPoints      []string  `json:"focus_points"`
	RiskLevel        string    `json:"risk_level"`
	RiskWarning      string    `json:"risk_warning"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AssetTimelineReq struct {
	InstrumentID string `form:"instrument_id" json:"instrument_id" uri:"instrument_id" binding:"required"`
	Period       string `form:"period" json:"period"`
	Limit        int    `form:"limit" json:"limit"`
	Locale       string `form:"locale" json:"locale"`
}

type AssetTimelineItem struct {
	EventID            string    `json:"event_id"`
	EventType          string    `json:"event_type"`
	Title              string    `json:"title"`
	Summary            string    `json:"summary"`
	Timestamp          time.Time `json:"timestamp"`
	ImpactLevel        string    `json:"impact_level"`
	SentimentDirection string    `json:"sentiment_direction"`
	MarkerPrice        *float64  `json:"marker_price"`
	MarkerStyle        string    `json:"marker_style"`
	RelatedNarrative   string    `json:"related_narrative"`
	RelatedSourceType  string    `json:"related_source_type"`
}

type AssetDigestReq struct {
	InstrumentID string `form:"instrument_id" json:"instrument_id" uri:"instrument_id" binding:"required"`
	Limit        int    `form:"limit" json:"limit"`
	Locale       string `form:"locale" json:"locale"`
}

type AssetDigestItem struct {
	DigestID          string    `json:"digest_id"`
	SourceType        string    `json:"source_type"`
	SourceName        string    `json:"source_name"`
	Title             string    `json:"title"`
	Summary           string    `json:"summary"`
	Sentiment         string    `json:"sentiment"`
	RelatedNarratives []string  `json:"related_narratives"`
	PublishedAt       time.Time `json:"published_at"`
	SourceURL         string    `json:"source_url"`
}
