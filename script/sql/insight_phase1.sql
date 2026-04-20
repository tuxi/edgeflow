CREATE TABLE IF NOT EXISTS `market_overview_snapshots` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `snapshot_time` DATETIME NOT NULL COMMENT '快照时间',
    `market_sentiment` VARCHAR(32) NOT NULL COMMENT '市场情绪',
    `risk_appetite` VARCHAR(32) NOT NULL COMMENT '风险偏好',
    `headline_narratives_json` JSON NULL COMMENT '热点叙事',
    `summary` TEXT NULL COMMENT '摘要文案',
    `leader_assets_json` JSON NULL COMMENT '代表资产',
    `source_window` VARCHAR(32) NOT NULL DEFAULT '24h' COMMENT '数据窗口',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_market_overview_snapshot_time` (`snapshot_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='市场概览快照';

CREATE TABLE IF NOT EXISTS `asset_insight_snapshots` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `sentiment_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '情绪分',
    `sentiment_change` DOUBLE NOT NULL DEFAULT 0 COMMENT '情绪变化',
    `attention_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '热度分',
    `attention_change` DOUBLE NOT NULL DEFAULT 0 COMMENT '热度变化',
    `mention_count` BIGINT NULL COMMENT '提及数',
    `mention_change_pct` DOUBLE NULL COMMENT '提及变化率',
    `narrative_tags_json` JSON NULL COMMENT '叙事标签',
    `risk_level` VARCHAR(32) NOT NULL DEFAULT 'medium' COMMENT '风险等级',
    `divergence_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '背离分',
    `reason_summary` TEXT NULL COMMENT '原因摘要',
    `focus_points_json` JSON NULL COMMENT '关注点',
    `risk_warning` TEXT NULL COMMENT '风险提示',
    `snapshot_time` DATETIME NOT NULL COMMENT '快照时间',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_asset_insight_instrument_snapshot_time` (`instrument_id`, `snapshot_time`),
    KEY `idx_asset_insight_symbol_snapshot_time` (`symbol`, `snapshot_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资产洞察快照';

CREATE TABLE IF NOT EXISTS `asset_timeline_events` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id` VARCHAR(64) NOT NULL COMMENT '事件唯一ID',
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `event_type` VARCHAR(32) NOT NULL COMMENT '事件类型',
    `title` VARCHAR(255) NOT NULL COMMENT '事件标题',
    `summary` TEXT NULL COMMENT '事件摘要',
    `impact_level` VARCHAR(32) NOT NULL DEFAULT 'medium' COMMENT '影响等级',
    `sentiment_direction` VARCHAR(32) NOT NULL DEFAULT 'neutral' COMMENT '情绪方向',
    `marker_price` DOUBLE NULL COMMENT '图表标记价格',
    `related_narrative` VARCHAR(64) NULL COMMENT '关联叙事',
    `related_source_type` VARCHAR(32) NULL COMMENT '关联来源类型',
    `event_time` DATETIME NOT NULL COMMENT '事件时间',
    `source_ref_type` VARCHAR(32) NULL COMMENT '来源引用类型',
    `source_ref_id` VARCHAR(64) NULL COMMENT '来源引用ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_asset_timeline_event_id` (`event_id`),
    KEY `idx_asset_timeline_instrument_event_time` (`instrument_id`, `event_time`),
    KEY `idx_asset_timeline_symbol_event_time` (`symbol`, `event_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资产时间线事件';

CREATE TABLE IF NOT EXISTS `asset_digest_items` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `digest_id` VARCHAR(64) NOT NULL COMMENT '摘要唯一ID',
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `source_type` VARCHAR(32) NOT NULL COMMENT '来源类型',
    `source_name` VARCHAR(64) NOT NULL COMMENT '来源名称',
    `title` VARCHAR(255) NOT NULL COMMENT '标题',
    `summary` TEXT NULL COMMENT '摘要内容',
    `sentiment` VARCHAR(32) NOT NULL DEFAULT 'neutral' COMMENT '情绪倾向',
    `related_narratives_json` JSON NULL COMMENT '关联叙事',
    `published_at` DATETIME NOT NULL COMMENT '发布时间',
    `source_url` VARCHAR(512) NULL COMMENT '来源URL',
    `raw_ref` VARCHAR(128) NULL COMMENT '原始引用ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_asset_digest_digest_id` (`digest_id`),
    KEY `idx_asset_digest_instrument_published_at` (`instrument_id`, `published_at`),
    KEY `idx_asset_digest_symbol_published_at` (`symbol`, `published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资产摘要条目';

CREATE TABLE IF NOT EXISTS `watchlist_candidates` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `group_type` VARCHAR(32) NOT NULL COMMENT '分组类型',
    `rank_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '排序分',
    `reason_summary` TEXT NULL COMMENT '原因摘要',
    `snapshot_ref_id` BIGINT UNSIGNED NULL COMMENT '关联快照ID',
    `window` VARCHAR(32) NOT NULL DEFAULT '24h' COMMENT '统计窗口',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_watchlist_group_rank` (`group_type`, `rank_score`),
    KEY `idx_watchlist_instrument_created_at` (`instrument_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关注候选列表';

CREATE TABLE IF NOT EXISTS `pipeline_run_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `pipeline_name` VARCHAR(64) NOT NULL COMMENT '流水线名称',
    `run_id` VARCHAR(64) NOT NULL COMMENT '运行ID',
    `status` VARCHAR(32) NOT NULL COMMENT '运行状态',
    `started_at` DATETIME NOT NULL COMMENT '开始时间',
    `finished_at` DATETIME NULL COMMENT '结束时间',
    `duration_ms` BIGINT NOT NULL DEFAULT 0 COMMENT '耗时毫秒',
    `processed_count` BIGINT NOT NULL DEFAULT 0 COMMENT '处理数',
    `success_count` BIGINT NOT NULL DEFAULT 0 COMMENT '成功数',
    `failed_count` BIGINT NOT NULL DEFAULT 0 COMMENT '失败数',
    `error_message` TEXT NULL COMMENT '错误信息',
    `meta_json` JSON NULL COMMENT '扩展元数据',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_pipeline_run_id` (`run_id`),
    KEY `idx_pipeline_name_started_at` (`pipeline_name`, `started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线运行日志';
