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
