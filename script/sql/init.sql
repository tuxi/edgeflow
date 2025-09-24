CREATE TABLE `order_record` (
                                `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                `order_id` VARCHAR(100) NOT NULL COMMENT '订单ID',
                                `symbol` VARCHAR(50) NOT NULL COMMENT '交易对',
                                `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                `timestamp` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '信号触发时间',
                                `side` VARCHAR(10) NOT NULL COMMENT '方向（buy/sell）',
                                `price` DOUBLE NOT NULL COMMENT '成交价格',
                                `quantity` DOUBLE NOT NULL COMMENT '数量',
                                `order_type` VARCHAR(20) NOT NULL COMMENT '订单类型（limit/market）',

                                `tp` DOUBLE DEFAULT NULL COMMENT '止盈价',
                                `sl` DOUBLE DEFAULT NULL COMMENT '止损价',

                                `strategy` VARCHAR(50) DEFAULT NULL COMMENT '策略名',
                                `comment` VARCHAR(255) DEFAULT NULL COMMENT '备注',

                                `trade_type` VARCHAR(20) DEFAULT NULL COMMENT '交易类型（开仓/平仓等）',
                                `mgn_mode` VARCHAR(20) DEFAULT NULL COMMENT '保证金模式（cross/isolate）',
                                `level` int(20) NOT NULL COMMENT '指标级别（1、2、3）',
                                `score` int(20) NOT NULL COMMENT '指标分数（1～10）',

                                PRIMARY KEY (`id`),
                                UNIQUE KEY `uk_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单记录表';

-- 币种表
CREATE TABLE `currencies` (
                              `id` BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '币ID，主键',
                              `ccy` VARCHAR(50) NOT NULL COMMENT '币符号，例如: BTC',
                              `name` VARCHAR(100) COMMENT '币中文名',
                              `name_en` VARCHAR(100) COMMENT '币英文名',
                              `logo_url` VARCHAR(255) COMMENT '币图标 URL',
                              `price_scale` INT DEFAULT 0 COMMENT '价格精度',
                              `status` INT DEFAULT 1 COMMENT '状态，1=可用，0=不可用',
                              `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                              `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                              UNIQUE KEY `uniq_ccy` (`ccy`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种表';

-- 交易所表
CREATE TABLE `exchanges` (
                             `id` BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '交易所ID',
                             `name` VARCHAR(100) NOT NULL COMMENT '交易所名称，例如: Binance',
                             `name_en` VARCHAR(100) COMMENT '交易所英文名',
                             UNIQUE KEY `uniq_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易所表';

-- 创建OKX和币安交易所
INSERT INTO `exchanges` (`name`, `name_en`) VALUES ('OKX', 'OKX');
INSERT INTO `exchanges` (`name`, `name_en`) VALUES ('币安', 'Binance');

-- 币种-交易所关联表
CREATE TABLE `exchange_currencies` (
                                       `id` BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
                                       `currency_id` BIGINT NOT NULL COMMENT '币种ID',
                                       `exchange_id` BIGINT NOT NULL COMMENT '交易所ID',
                                       `is_active` TINYINT(1) DEFAULT 1 COMMENT '该币在交易所是否可用',
                                       UNIQUE KEY `uniq_currency_exchange` (`currency_id`, `exchange_id`),
                                       KEY `idx_exchange_active` (`exchange_id`, `is_active`),
                                       CONSTRAINT `fk_currency` FOREIGN KEY (`currency_id`) REFERENCES `currencies`(`id`) ON DELETE CASCADE ON UPDATE CASCADE,
                                       CONSTRAINT `fk_exchange` FOREIGN KEY (`exchange_id`) REFERENCES `exchanges`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种和交易所关联表';

CREATE TABLE `whale` (
                       id BIGINT AUTO_INCREMENT PRIMARY KEY,
                       address VARCHAR(64) NOT NULL UNIQUE,
                       display_name VARCHAR(100),
                       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

                       UNIQUE KEY `uniq_address` (`address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鲸鱼';


CREATE TABLE `hyper_whale_leaderboard` (
                                   id BIGINT AUTO_INCREMENT PRIMARY KEY,
                                   address VARCHAR(64) NOT NULL,
                                   account_value DECIMAL(32,10) NOT NULL,     -- 账户价值
    -- PnL 数据
                                   pnl_day DECIMAL(32,10),
                                   pnl_week DECIMAL(32,10),
                                   pnl_month DECIMAL(32,10),
                                   pnl_all_time DECIMAL(32,10),

    -- ROI 数据
                                   roi_day DECIMAL(16,10),
                                   roi_week DECIMAL(16,10),
                                   roi_month DECIMAL(16,10),
                                   roi_all_time DECIMAL(16,10),

    -- VLM 数据（交易量）
                                   vlm_day DECIMAL(32,10),
                                   vlm_week DECIMAL(32,10),
                                   vlm_month DECIMAL(32,10),
                                   vlm_all_time DECIMAL(32,10),

                                   `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                   `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

                                    UNIQUE KEY `uniq_address` (`address`)
#                                    FOREIGN KEY (address) REFERENCES whale(address) -- 不需要外键
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='hyper鲸鱼排行';