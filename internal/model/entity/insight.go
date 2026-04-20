package entity

import "time"

type MarketOverviewSnapshot struct {
	ID                     uint64    `gorm:"primaryKey;column:id" json:"id"`
	SnapshotTime           time.Time `gorm:"column:snapshot_time" json:"snapshot_time"`
	MarketSentiment        string    `gorm:"column:market_sentiment" json:"market_sentiment"`
	RiskAppetite           string    `gorm:"column:risk_appetite" json:"risk_appetite"`
	HeadlineNarrativesJSON string    `gorm:"column:headline_narratives_json" json:"headline_narratives_json"`
	Summary                string    `gorm:"column:summary" json:"summary"`
	LeaderAssetsJSON       string    `gorm:"column:leader_assets_json" json:"leader_assets_json"`
	SourceWindow           string    `gorm:"column:source_window" json:"source_window"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at"`
}

func (MarketOverviewSnapshot) TableName() string {
	return "market_overview_snapshots"
}

type AssetInsightSnapshot struct {
	ID                uint64    `gorm:"primaryKey;column:id" json:"id"`
	InstrumentID      string    `gorm:"column:instrument_id" json:"instrument_id"`
	Symbol            string    `gorm:"column:symbol" json:"symbol"`
	SentimentScore    float64   `gorm:"column:sentiment_score" json:"sentiment_score"`
	SentimentChange   float64   `gorm:"column:sentiment_change" json:"sentiment_change"`
	AttentionScore    float64   `gorm:"column:attention_score" json:"attention_score"`
	AttentionChange   float64   `gorm:"column:attention_change" json:"attention_change"`
	MentionCount      *int64    `gorm:"column:mention_count" json:"mention_count"`
	MentionChangePct  *float64  `gorm:"column:mention_change_pct" json:"mention_change_pct"`
	NarrativeTagsJSON string    `gorm:"column:narrative_tags_json" json:"narrative_tags_json"`
	RiskLevel         string    `gorm:"column:risk_level" json:"risk_level"`
	DivergenceScore   float64   `gorm:"column:divergence_score" json:"divergence_score"`
	ReasonSummary     string    `gorm:"column:reason_summary" json:"reason_summary"`
	FocusPointsJSON   string    `gorm:"column:focus_points_json" json:"focus_points_json"`
	RiskWarning       string    `gorm:"column:risk_warning" json:"risk_warning"`
	SnapshotTime      time.Time `gorm:"column:snapshot_time" json:"snapshot_time"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AssetInsightSnapshot) TableName() string {
	return "asset_insight_snapshots"
}

type AssetTimelineEvent struct {
	ID                 uint64    `gorm:"primaryKey;column:id" json:"id"`
	EventID            string    `gorm:"column:event_id" json:"event_id"`
	InstrumentID       string    `gorm:"column:instrument_id" json:"instrument_id"`
	Symbol             string    `gorm:"column:symbol" json:"symbol"`
	EventType          string    `gorm:"column:event_type" json:"event_type"`
	Title              string    `gorm:"column:title" json:"title"`
	Summary            string    `gorm:"column:summary" json:"summary"`
	ImpactLevel        string    `gorm:"column:impact_level" json:"impact_level"`
	SentimentDirection string    `gorm:"column:sentiment_direction" json:"sentiment_direction"`
	MarkerPrice        *float64  `gorm:"column:marker_price" json:"marker_price"`
	RelatedNarrative   string    `gorm:"column:related_narrative" json:"related_narrative"`
	RelatedSourceType  string    `gorm:"column:related_source_type" json:"related_source_type"`
	EventTime          time.Time `gorm:"column:event_time" json:"event_time"`
	SourceRefType      string    `gorm:"column:source_ref_type" json:"source_ref_type"`
	SourceRefID        string    `gorm:"column:source_ref_id" json:"source_ref_id"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AssetTimelineEvent) TableName() string {
	return "asset_timeline_events"
}

type AssetDigestItem struct {
	ID                    uint64    `gorm:"primaryKey;column:id" json:"id"`
	DigestID              string    `gorm:"column:digest_id" json:"digest_id"`
	InstrumentID          string    `gorm:"column:instrument_id" json:"instrument_id"`
	Symbol                string    `gorm:"column:symbol" json:"symbol"`
	SourceType            string    `gorm:"column:source_type" json:"source_type"`
	SourceName            string    `gorm:"column:source_name" json:"source_name"`
	Title                 string    `gorm:"column:title" json:"title"`
	Summary               string    `gorm:"column:summary" json:"summary"`
	Sentiment             string    `gorm:"column:sentiment" json:"sentiment"`
	RelatedNarrativesJSON string    `gorm:"column:related_narratives_json" json:"related_narratives_json"`
	PublishedAt           time.Time `gorm:"column:published_at" json:"published_at"`
	SourceURL             string    `gorm:"column:source_url" json:"source_url"`
	RawRef                string    `gorm:"column:raw_ref" json:"raw_ref"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AssetDigestItem) TableName() string {
	return "asset_digest_items"
}

type WatchlistCandidate struct {
	ID            uint64    `gorm:"primaryKey;column:id" json:"id"`
	InstrumentID  string    `gorm:"column:instrument_id" json:"instrument_id"`
	Symbol        string    `gorm:"column:symbol" json:"symbol"`
	GroupType     string    `gorm:"column:group_type" json:"group_type"`
	RankScore     float64   `gorm:"column:rank_score" json:"rank_score"`
	ReasonSummary string    `gorm:"column:reason_summary" json:"reason_summary"`
	SnapshotRefID *uint64   `gorm:"column:snapshot_ref_id" json:"snapshot_ref_id"`
	Window        string    `gorm:"column:window" json:"window"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (WatchlistCandidate) TableName() string {
	return "watchlist_candidates"
}
