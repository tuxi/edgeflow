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


CREATE TABLE `hyper_whale_position` (
                                   `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                   `address` VARCHAR(100) NOT NULL COMMENT '用户地址',
                                   `coin` VARCHAR(50) NOT NULL COMMENT '币种',
                                   `type` VARCHAR(20) NOT NULL COMMENT '仓位类型(oneWay单向/twoWay双向)',
                                   `side` VARCHAR(20) NOT NULL COMMENT '仓位方向(long/short)',
                                   `entry_px` DECIMAL(40, 18) DEFAULT NULL COMMENT '开仓价格',
                                    `liquidation_px` DECIMAL(40, 18) DEFAULT NULL COMMENT '强平价格',
                                   `position_value` DECIMAL(40, 18) DEFAULT NULL COMMENT '仓位价值',
                                   `margin_used` DECIMAL(40, 18) DEFAULT NULL COMMENT '保证金',
                                   `funding_fee` DECIMAL(40, 18) DEFAULT NULL COMMENT '资金费',
                                   `szi` DECIMAL(40, 18) DEFAULT NULL COMMENT '仓位数量',
                                   `unrealized_pnl` VARCHAR(50) DEFAULT NULL COMMENT '未实现盈亏',
                                   `return_on_equity` VARCHAR(50) DEFAULT NULL COMMENT 'ROE',
                                   `leverage_type` VARCHAR(20) NOT NULL COMMENT '杠杆模式( isolated / cross )',
                                   `leverage_value` INT NOT NULL COMMENT '杠杆倍数',
                                   `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                   `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                   PRIMARY KEY (`id`),
                                   UNIQUE KEY `idx_whale_position_unique` (`address`, `coin`, `side`, `leverage_type`, `leverage_value`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鲸鱼仓位表';


CREATE TABLE `crypto_exchanges` (
                                    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                    `code` VARCHAR(16) NOT NULL UNIQUE COMMENT '交易所代码 (如 OKX, BINANCE)',
                                    `name` VARCHAR(64) NOT NULL COMMENT '交易所名称',
                                    `status` VARCHAR(16) NOT NULL COMMENT '服务状态 (ACTIVE, MAINTENANCE)',
                                    `api_endpoint` VARCHAR(255) NULL COMMENT '基础 API 地址',
                                    `country` VARCHAR(32) NULL COMMENT '运营国家/地区',
                                    `created_at` DATETIME NOT NULL COMMENT '创建时间',
                                    `updated_at` DATETIME NOT NULL COMMENT '最后更新时间',
                                    PRIMARY KEY (`id`),
                                    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易所元数据表';

-- 1. 交易对元数据表
CREATE TABLE `crypto_instruments` (
                                      `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                      -- instrument_id 不再是 UNIQUE
                                      `instrument_id` VARCHAR(32) NOT NULL COMMENT '交易所交易对ID (如 BTC-USDT)',
                                      `exchange_id`	INT UNSIGNED NOT NULL COMMENT '关联到 crypto_exchanges.id',
                                      `base_ccy` VARCHAR(16) NOT NULL COMMENT '主币代码 (如 BTC)',
                                      `quote_ccy` VARCHAR(16) NOT NULL COMMENT '计价币代码 (如 USDT)',
                                      `name_cn` VARCHAR(64) NOT NULL COMMENT '中文名称',
                                      `name_en` VARCHAR(64) NOT NULL COMMENT '英文名称',
                                      `status` VARCHAR(16) NOT NULL COMMENT '交易状态 (如 LIVE, DELIST)',
                                      `price_precision` DECIMAL(40, 18) NOT NULL COMMENT '价格精度/步长 (tickSz)',
                                      `qty_precision` DECIMAL(40, 18) NOT NULL COMMENT '数量/合约精度 (szIncrement)',
                                      `market_cap` BIGINT UNSIGNED DEFAULT 0 COMMENT '市值 (USD)',
                                      `is_contract` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为合约标的 (0否, 1是)',
                                      `created_at` DATETIME NOT NULL COMMENT '创建时间',
                                      `updated_at` DATETIME NOT NULL COMMENT '最后更新时间',
                                        -- 核心：定义联合主键
                                        -- 此时，单独的 id 字段实际上成了“代理主键”，而联合主键才是业务逻辑上的唯一标识
                                      PRIMARY KEY (`id`),
                                        -- 关键：为联合主键创建唯一索引，保证业务上的唯一性
                                      UNIQUE KEY `uk_exchange_instrument` (`exchange_id`, `instrument_id`),
                                    -- 辅助索引
                                      KEY `idx_base_ccy` (`base_ccy`),
                                      KEY `idx_status` (`status`),
                                      -- 建立外键关联
                                     FOREIGN KEY (`exchange_id`) REFERENCES `crypto_exchanges`(`id`) ON DELETE CASCADE

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种及交易对元数据表';

-- 2. 标签定义表
CREATE TABLE `crypto_tags` (
                               `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '标签ID',
                               `name` VARCHAR(32) NOT NULL UNIQUE COMMENT '标签名称 (如 MEME)',
                               `description` VARCHAR(128) NULL COMMENT '标签描述',
                               PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种标签定义表';

-- 3. 交易对和标签关系表
CREATE TABLE `instrument_tag_relations` (
                                            `instrument_id` BIGINT UNSIGNED NOT NULL COMMENT '关联到 crypto_instruments.id',
                                            `tag_id` INT UNSIGNED NOT NULL COMMENT '关联到 crypto_tags.id',
                                            PRIMARY KEY (`instrument_id`, `tag_id`),
                                            FOREIGN KEY (`instrument_id`) REFERENCES `crypto_instruments`(`id`) ON DELETE CASCADE,
                                            FOREIGN KEY (`tag_id`) REFERENCES `crypto_tags`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易对-标签关系表';
