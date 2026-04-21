CREATE TABLE IF NOT EXISTS `signal_setups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `setup_key` VARCHAR(128) NOT NULL COMMENT 'setup幂等键',
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `period` VARCHAR(32) NOT NULL COMMENT '周期',
    `setup_type` VARCHAR(64) NOT NULL COMMENT 'setup类型',
    `direction` VARCHAR(32) NOT NULL COMMENT '方向',
    `trigger_bar_time` DATETIME NOT NULL COMMENT '触发bar时间',
    `entry_anchor_price` DOUBLE NOT NULL DEFAULT 0 COMMENT '入场锚点价',
    `trigger_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '触发分',
    `indicator_snapshot_json` JSON NULL COMMENT '指标快照',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'setup状态',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_signal_setup_key` (`setup_key`),
    KEY `idx_signal_setup_instrument_trigger_time` (`instrument_id`, `trigger_bar_time`),
    KEY `idx_signal_setup_symbol_status` (`symbol`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='信号候选setup';

CREATE TABLE IF NOT EXISTS `signal_clusters` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `cluster_key` VARCHAR(128) NOT NULL COMMENT 'cluster幂等键',
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `direction` VARCHAR(32) NOT NULL COMMENT '方向',
    `cluster_type` VARCHAR(64) NOT NULL COMMENT 'cluster类型',
    `first_trigger_time` DATETIME NOT NULL COMMENT '首次触发时间',
    `last_trigger_time` DATETIME NOT NULL COMMENT '最后触发时间',
    `entry_anchor_price` DOUBLE NOT NULL DEFAULT 0 COMMENT '入场锚点价',
    `marker_price` DOUBLE NULL COMMENT '图表标记价格',
    `signal_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计信号数',
    `priority` INT NOT NULL DEFAULT 0 COMMENT '展示优先级',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'cluster状态',
    `latest_setup_ref_id` BIGINT UNSIGNED NULL COMMENT '最近setup引用',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_signal_cluster_key` (`cluster_key`),
    KEY `idx_signal_cluster_instrument_last_trigger` (`instrument_id`, `last_trigger_time`),
    KEY `idx_signal_cluster_symbol_status` (`symbol`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='信号聚合cluster';

CREATE TABLE IF NOT EXISTS `decision_records` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `instrument_id` VARCHAR(64) NOT NULL COMMENT '交易对ID',
    `symbol` VARCHAR(32) NOT NULL COMMENT '资产符号',
    `setup_ref_id` BIGINT UNSIGNED NULL COMMENT 'setup引用',
    `regime_ref_id` BIGINT UNSIGNED NULL COMMENT 'regime引用',
    `decision_type` VARCHAR(32) NOT NULL COMMENT '决策类型',
    `reason_code` VARCHAR(64) NOT NULL COMMENT '原因编码',
    `cooldown_until` DATETIME NULL COMMENT '冷却结束时间',
    `cluster_key` VARCHAR(128) NULL COMMENT 'cluster键',
    `risk_gate` VARCHAR(32) NULL COMMENT '风险闸门',
    `payload_json` JSON NULL COMMENT '扩展载荷',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_decision_record_instrument_created_at` (`instrument_id`, `created_at`),
    KEY `idx_decision_record_symbol_created_at` (`symbol`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='策略决策记录';

SET @setup_key_exists = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'signals'
      AND COLUMN_NAME = 'setup_key'
);
SET @setup_key_sql = IF(
    @setup_key_exists = 0,
    'ALTER TABLE `signals` ADD COLUMN `setup_key` VARCHAR(128) NULL COMMENT ''兼容setup键''',
    'SELECT 1'
);
PREPARE stmt FROM @setup_key_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @cluster_key_exists = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'signals'
      AND COLUMN_NAME = 'cluster_key'
);
SET @cluster_key_sql = IF(
    @cluster_key_exists = 0,
    'ALTER TABLE `signals` ADD COLUMN `cluster_key` VARCHAR(128) NULL COMMENT ''兼容cluster键''',
    'SELECT 1'
);
PREPARE stmt FROM @cluster_key_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @decision_ref_exists = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'signals'
      AND COLUMN_NAME = 'decision_ref_id'
);
SET @decision_ref_sql = IF(
    @decision_ref_exists = 0,
    'ALTER TABLE `signals` ADD COLUMN `decision_ref_id` BIGINT UNSIGNED NULL COMMENT ''兼容决策引用''',
    'SELECT 1'
);
PREPARE stmt FROM @decision_ref_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
