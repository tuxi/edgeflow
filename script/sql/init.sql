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

CREATE TABLE `coin_categories` (
                                   `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                   `name` VARCHAR(50) NOT NULL COMMENT '中文名称 (主流币 / 公链 / DeFi)',
                                   `name_en` VARCHAR(50) DEFAULT NULL COMMENT '英文名称 (Mainstream / Chain / DeFi)',
                                   `logo_url` VARCHAR(255) DEFAULT NULL COMMENT '分类图标',
                                   `status` TINYINT DEFAULT 1 COMMENT '状态 1=启用, 0=禁用',
                                   `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                   `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                   PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种分类表';

/*
INSERT INTO coin_categories (`name`, `name_en`, `logo_url`, `status`)
VALUES ('主流币', 'Mainstream', 'https://example.com/logo/mainstream.png', 1);
*/


CREATE TABLE `coins` (
                         `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                         `coin` VARCHAR(50) NOT NULL COMMENT '币符号（BTC）',
                         `name` VARCHAR(50) DEFAULT NULL COMMENT '中文名称 (比特币)',
                         `name_en` VARCHAR(50) DEFAULT NULL COMMENT '英文名称 (Bitcoin)',
                         `logo_url` VARCHAR(255) COMMENT '图标',
                         `category_id` BIGINT UNSIGNED NOT NULL COMMENT '分类ID，关联 coin_category.id',
                         `price_scale` INT COMMENT '价格小数位精度 (例: 4代表4位小数)',
                         `status` TINYINT DEFAULT 1 COMMENT '状态 1=上线, 0=下架',
                         `source` VARCHAR(20) DEFAULT 'OKX' COMMENT '默认交易所',
                         `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                         `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                         PRIMARY KEY (`id`),
                         KEY `idx_coin_symbol` (`coin`),
                         CONSTRAINT `fk_coins_category` FOREIGN KEY (`category_id`) REFERENCES `coin_categories` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='币种表';

/*
INSERT INTO coins (`coin`, `name`, `name_en`, `logo_url`, `category_id`, `price_scale`, `status`, `source`)
VALUES ('BTC', '比特币', 'Bitcoin', 'https://example.com/logo/btc.png', 1, 2, 1, 'OKX');
*/