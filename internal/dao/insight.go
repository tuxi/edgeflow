package dao

import (
	"context"
	"edgeflow/internal/model/entity"
	"time"
)

type InsightDao interface {
	GetLatestMarketOverview(ctx context.Context) (*entity.MarketOverviewSnapshot, error)
	GetLatestAssetInsightSnapshot(ctx context.Context, instrumentID string) (*entity.AssetInsightSnapshot, error)
	ListTimelineEvents(ctx context.Context, instrumentID string, limit int, since *time.Time) ([]entity.AssetTimelineEvent, error)
	ListDigestItems(ctx context.Context, instrumentID string, limit int) ([]entity.AssetDigestItem, error)
	ListWatchlistCandidates(ctx context.Context, group string, limit int) ([]entity.WatchlistCandidate, error)
}
