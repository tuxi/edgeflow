package service

import (
	"context"
	"edgeflow/internal/consts"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"encoding/json"
	gerrors "errors"
	"gorm.io/gorm"
	"sort"
	"time"
)

type InsightService struct {
	dao dao.InsightDao
}

func NewInsightService(dao dao.InsightDao) *InsightService {
	return &InsightService{dao: dao}
}

func (s *InsightService) resolveLocale(ctx context.Context, preferred string) string {
	if preferred != "" {
		return normalizeLocale(preferred)
	}
	if value, ok := ctx.Value(consts.LanguageId).(string); ok {
		return normalizeLocale(value)
	}
	return defaultLocale
}

func (s *InsightService) GetMarketOverview(ctx context.Context, preferredLocale string) (*model.MarketOverviewRes, error) {
	locale := s.resolveLocale(ctx, preferredLocale)

	snapshot, err := s.dao.GetLatestMarketOverview(ctx)
	if err != nil {
		if gerrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithCode(ecode.NotFoundErr, "insight market overview not found")
		}
		return nil, err
	}

	var narratives []model.HeadlineNarrative
	var leaders []string
	_ = json.Unmarshal([]byte(snapshot.HeadlineNarrativesJSON), &narratives)
	_ = json.Unmarshal([]byte(snapshot.LeaderAssetsJSON), &leaders)
	narratives = localizeHeadlineNarratives(locale, narratives)

	return &model.MarketOverviewRes{
		Sentiment:          snapshot.MarketSentiment,
		RiskAppetite:       snapshot.RiskAppetite,
		HeadlineNarratives: narratives,
		Summary:            buildMarketOverviewSummary(locale, snapshot.MarketSentiment, snapshot.RiskAppetite, narratives),
		LeaderAssets:       leaders,
		UpdatedAt:          snapshot.SnapshotTime,
	}, nil
}

func (s *InsightService) GetMarketWatchlist(ctx context.Context, req model.MarketWatchlistReq) ([]model.MarketWatchlistItem, error) {
	locale := s.resolveLocale(ctx, req.Locale)

	candidates, err := s.dao.ListWatchlistCandidates(ctx, req.Group, req.Limit)
	if err != nil {
		return nil, err
	}

	items := make([]model.MarketWatchlistItem, 0, len(candidates))
	for _, candidate := range candidates {
		snapshot, snapErr := s.dao.GetLatestAssetInsightSnapshot(ctx, candidate.InstrumentID)
		if snapErr != nil {
			if gerrors.Is(snapErr, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, snapErr
		}

		var tags []string
		_ = json.Unmarshal([]byte(snapshot.NarrativeTagsJSON), &tags)

		items = append(items, model.MarketWatchlistItem{
			InstrumentID:    candidate.InstrumentID,
			Symbol:          candidate.Symbol,
			GroupType:       candidate.GroupType,
			RankScore:       candidate.RankScore,
			ReasonSummary:   buildWatchlistReason(locale, candidate.GroupType, snapshot),
			SentimentScore:  snapshot.SentimentScore,
			SentimentChange: snapshot.SentimentChange,
			AttentionScore:  snapshot.AttentionScore,
			AttentionChange: snapshot.AttentionChange,
			NarrativeTags:   tags,
			RiskLevel:       snapshot.RiskLevel,
			DivergenceScore: snapshot.DivergenceScore,
			UpdatedAt:       snapshot.SnapshotTime,
		})
	}
	return items, nil
}

func (s *InsightService) GetAssetSummary(ctx context.Context, instrumentID string, preferredLocale string) (*model.AssetInsightSummaryRes, error) {
	locale := s.resolveLocale(ctx, preferredLocale)

	snapshot, err := s.dao.GetLatestAssetInsightSnapshot(ctx, instrumentID)
	if err != nil {
		if gerrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithCode(ecode.NotFoundErr, "asset insight summary not found")
		}
		return nil, err
	}

	var tags []string
	var focus []string
	_ = json.Unmarshal([]byte(snapshot.NarrativeTagsJSON), &tags)
	_ = json.Unmarshal([]byte(snapshot.FocusPointsJSON), &focus)
	focus = buildFocusPoints(locale, snapshot, tags)

	return &model.AssetInsightSummaryRes{
		InstrumentID:     snapshot.InstrumentID,
		Symbol:           snapshot.Symbol,
		SentimentScore:   snapshot.SentimentScore,
		AttentionScore:   snapshot.AttentionScore,
		MentionCount:     snapshot.MentionCount,
		MentionChangePct: snapshot.MentionChangePct,
		NarrativeTags:    tags,
		Summary:          buildAssetSummaryText(locale, snapshot, tags),
		FocusPoints:      focus,
		RiskLevel:        snapshot.RiskLevel,
		RiskWarning:      buildRiskWarning(locale, snapshot),
		UpdatedAt:        snapshot.SnapshotTime,
	}, nil
}

func (s *InsightService) GetAssetTimeline(ctx context.Context, instrumentID string, period string, limit int, preferredLocale string) ([]model.AssetTimelineItem, error) {
	locale := s.resolveLocale(ctx, preferredLocale)
	since, err := resolveTimelineSince(period)
	if err != nil {
		return nil, err
	}

	events, err := s.dao.ListTimelineEvents(ctx, instrumentID, limit, since)
	if err != nil {
		return nil, err
	}

	items := make([]model.AssetTimelineItem, 0, len(events))
	for _, event := range events {
		items = append(items, localizeTimelineItem(locale, model.AssetTimelineItem{
			EventID:            event.EventID,
			EventType:          event.EventType,
			Title:              event.Title,
			Summary:            event.Summary,
			Timestamp:          event.EventTime,
			ImpactLevel:        event.ImpactLevel,
			SentimentDirection: event.SentimentDirection,
			MarkerPrice:        event.MarkerPrice,
			RelatedNarrative:   event.RelatedNarrative,
			RelatedSourceType:  event.RelatedSourceType,
		}))
	}
	return sortTimelineItems(dedupeTimelineItems(items)), nil
}

func resolveTimelineSince(period string) (*time.Time, error) {
	now := time.Now()
	switch period {
	case "", "7d":
		since := now.Add(-7 * 24 * time.Hour)
		return &since, nil
	case "24h":
		since := now.Add(-24 * time.Hour)
		return &since, nil
	default:
		return nil, errors.WithCode(ecode.ValidateErr, "period must be one of: 24h, 7d")
	}
}

func (s *InsightService) GetAssetDigest(ctx context.Context, instrumentID string, limit int, preferredLocale string) ([]model.AssetDigestItem, error) {
	locale := s.resolveLocale(ctx, preferredLocale)

	list, err := s.dao.ListDigestItems(ctx, instrumentID, limit)
	if err != nil {
		return nil, err
	}

	items := make([]model.AssetDigestItem, 0, len(list))
	for _, item := range list {
		var narratives []string
		_ = json.Unmarshal([]byte(item.RelatedNarrativesJSON), &narratives)
		items = append(items, localizeDigestItem(locale, model.AssetDigestItem{
			DigestID:          item.DigestID,
			SourceType:        item.SourceType,
			SourceName:        item.SourceName,
			Title:             item.Title,
			Summary:           item.Summary,
			Sentiment:         item.Sentiment,
			RelatedNarratives: narratives,
			PublishedAt:       item.PublishedAt,
			SourceURL:         item.SourceURL,
		}))
	}
	return sortDigestItems(dedupeDigestItems(items)), nil
}

func dedupeTimelineItems(items []model.AssetTimelineItem) []model.AssetTimelineItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]model.AssetTimelineItem, 0, len(items))
	for _, item := range items {
		key := item.EventID
		if item.EventType == "whale_activity" {
			key = "whale_" + item.Timestamp.Truncate(time.Hour).Format(time.RFC3339)
		}
		if item.EventType == "signal_created" {
			key = "signal_" + item.SentimentDirection + "_" + item.Timestamp.Truncate(time.Hour).Format(time.RFC3339)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeDigestItems(items []model.AssetDigestItem) []model.AssetDigestItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]model.AssetDigestItem, 0, len(items))
	for _, item := range items {
		key := item.DigestID
		if item.SourceType == "whale" {
			key = "whale_" + item.PublishedAt.Truncate(time.Hour).Format(time.RFC3339)
		}
		if item.SourceType == "signal" {
			key = "signal_" + item.Sentiment + "_" + item.PublishedAt.Truncate(time.Hour).Format(time.RFC3339)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sortTimelineItems(items []model.AssetTimelineItem) []model.AssetTimelineItem {
	sort.SliceStable(items, func(i, j int) bool {
		leftBucket := items[i].Timestamp.Truncate(time.Hour)
		rightBucket := items[j].Timestamp.Truncate(time.Hour)
		if !leftBucket.Equal(rightBucket) {
			return leftBucket.After(rightBucket)
		}
		leftPriority := timelinePriority(items[i])
		rightPriority := timelinePriority(items[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return items[i].Timestamp.After(items[j].Timestamp)
	})
	return items
}

func sortDigestItems(items []model.AssetDigestItem) []model.AssetDigestItem {
	sort.SliceStable(items, func(i, j int) bool {
		leftBucket := items[i].PublishedAt.Truncate(time.Hour)
		rightBucket := items[j].PublishedAt.Truncate(time.Hour)
		if !leftBucket.Equal(rightBucket) {
			return leftBucket.After(rightBucket)
		}
		leftPriority := digestPriority(items[i])
		rightPriority := digestPriority(items[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	return items
}

func timelinePriority(item model.AssetTimelineItem) int {
	switch item.EventType {
	case "signal_created":
		return 0
	case "risk_warning":
		return 1
	case "whale_activity":
		return 2
	default:
		return 3
	}
}

func digestPriority(item model.AssetDigestItem) int {
	switch item.SourceType {
	case "signal":
		return 0
	case "system":
		return 1
	case "whale":
		return 2
	default:
		return 3
	}
}
