package query

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
	"time"
)

type insightDao struct {
	db *gorm.DB
}

func NewInsightDao(db *gorm.DB) dao.InsightDao {
	return &insightDao{db: db}
}

func (d *insightDao) GetLatestMarketOverview(ctx context.Context) (*entity.MarketOverviewSnapshot, error) {
	var snapshot entity.MarketOverviewSnapshot
	err := d.db.WithContext(ctx).
		Order("snapshot_time DESC").
		First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (d *insightDao) GetLatestAssetInsightSnapshot(ctx context.Context, instrumentID string) (*entity.AssetInsightSnapshot, error) {
	var snapshot entity.AssetInsightSnapshot
	err := d.db.WithContext(ctx).
		Where("instrument_id = ?", instrumentID).
		Order("snapshot_time DESC").
		First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (d *insightDao) ListTimelineEvents(ctx context.Context, instrumentID string, limit int, since *time.Time) ([]entity.AssetTimelineEvent, error) {
	var list []entity.AssetTimelineEvent
	if limit <= 0 {
		limit = 20
	}
	tx := d.db.WithContext(ctx).
		Where("instrument_id = ?", instrumentID)
	if since != nil {
		tx = tx.Where("event_time >= ?", *since)
	}
	err := tx.Order("event_time DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

func (d *insightDao) ListDigestItems(ctx context.Context, instrumentID string, limit int) ([]entity.AssetDigestItem, error) {
	var list []entity.AssetDigestItem
	if limit <= 0 {
		limit = 20
	}
	err := d.db.WithContext(ctx).
		Where("instrument_id = ?", instrumentID).
		Order("published_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

func (d *insightDao) ListWatchlistCandidates(ctx context.Context, group string, limit int) ([]entity.WatchlistCandidate, error) {
	var list []entity.WatchlistCandidate
	if limit <= 0 {
		limit = 20
	}
	tx := d.db.WithContext(ctx).Model(&entity.WatchlistCandidate{})
	if group != "" {
		tx = tx.Where("group_type = ?", group)
	}
	err := tx.Order("rank_score DESC").Limit(limit).Find(&list).Error
	return list, err
}
