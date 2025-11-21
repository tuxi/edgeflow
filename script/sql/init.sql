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

CREATE TABLE `whale` (
                       id BIGINT AUTO_INCREMENT PRIMARY KEY,
                       address VARCHAR(64) NOT NULL UNIQUE,
                       display_name VARCHAR(100),
                       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

                       UNIQUE KEY `uniq_address` (`address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鲸鱼';
ALTER TABLE `whale`
    ADD COLUMN `source` VARCHAR(32) COMMENT '来源' AFTER `address`,
    ADD COLUMN `weight` INT DEFAULT 0 COMMENT '权重' AFTER `source`;

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

# 增加一些自主计算胜率的字段
ALTER TABLE `hyper_whale_leaderboard`
    ADD COLUMN `win_rate_updated_at` datetime NULL COMMENT '胜率数据最后更新时间' AFTER `address`,
    ADD COLUMN `win_rate_calc_start_time` bigint(20) NULL COMMENT '开始统计胜率的时间' AFTER `win_rate_updated_at`,
    ADD COLUMN `last_successful_time` bigint(20) NULL COMMENT '最后成功保存胜率快照的时间' AFTER `win_rate_calc_start_time`,
    ADD COLUMN `total_accumulated_trades` bigint(20) NULL COMMENT '总平仓笔数' AFTER `last_successful_time`,
    ADD COLUMN `total_accumulated_profits` bigint(20) NULL COMMENT '总的盈利单数' AFTER `total_accumulated_trades`,
    ADD COLUMN `total_accumulated_pnl` double NULL COMMENT '已实现的总盈利' AFTER `total_accumulated_profits`;


CREATE TABLE `whale_winrate_snapshots` (
                                           `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
                                           `address` VARCHAR(64) NOT NULL COMMENT '鲸鱼地址',
                                           `start_time` DATETIME NOT NULL COMMENT '本次计算的开始时间 (用于聚合查询)',
                                           `end_time` DATETIME NOT NULL COMMENT '本次计算的结束时间 (用于增量基线)',
                                           `total_closed_trades` BIGINT NOT NULL COMMENT '该时间块的总平仓笔数',
                                           `winning_trades` BIGINT NOT NULL COMMENT '该时间块的盈利笔数',
                                           `total_pnl` DOUBLE NOT NULL COMMENT '该时间块的总PnL',
                                           `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
                                           PRIMARY KEY (`id`),

                                           INDEX `idx_address` (`address`),
                                           INDEX `idx_start_time` (`start_time`),
                                           INDEX `idx_end_time` (`end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='鲸鱼胜率时间块快照表';

CREATE TABLE `hyper_whale_position` (
                                   `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                   `address` VARCHAR(64) NOT NULL COMMENT '用户地址',
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

-- 注意：建议使用 utf8mb4 编码
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- 用户信息表
-- ----------------------------
DROP TABLE IF EXISTS `user`;
CREATE TABLE `user` (
                        `id` BIGINT NOT NULL COMMENT '用户唯一ID',
                        `username` VARCHAR(255) NOT NULL COMMENT '用户名，唯一且不能为空',
                        `nickname` VARCHAR(255) NOT NULL COMMENT '昵称',
                        `email` VARCHAR(255) DEFAULT NULL COMMENT '邮件地址',
                        `phone` VARCHAR(255) DEFAULT NULL COMMENT '手机号',
                        `avatar_url` VARCHAR(255) DEFAULT NULL COMMENT '头像 URL',
                        `password` VARCHAR(255) DEFAULT NULL COMMENT '密码',
                        `registered_ip` VARCHAR(255) NOT NULL COMMENT '用户的注册 IP 地址',
                        `is_active` BOOLEAN NOT NULL DEFAULT FALSE COMMENT '用户是否已激活',
                        `balance` DECIMAL(10,5) DEFAULT 0 COMMENT '用户的余额，默认为0',
                        `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录的创建时间，默认为当前时间',
                        `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录的更新时间，默认为当前时间',
                        `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间',
                        `is_del` INT DEFAULT 0 COMMENT '删除标志',
                        `is_anonymous` BOOLEAN DEFAULT FALSE COMMENT '是否为匿名用户',
                        `role` INT NOT NULL DEFAULT 1 COMMENT '用户角色',
                        `is_administrator` BOOLEAN DEFAULT FALSE COMMENT '是否为管理员',
                        PRIMARY KEY (`id`),
                        UNIQUE KEY `user_username_uindex` (`username`),
                        KEY `user_email_idx` (`email`),
                        KEY `user_username_idx` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户信息表';

-- ----------------------------
-- 设备信息表
-- ----------------------------
DROP TABLE IF EXISTS `device`;
CREATE TABLE `device` (
                          `id` BIGINT NOT NULL COMMENT '主键id',
                          `uuid` VARCHAR(255) NOT NULL COMMENT '设备uuid，要符合UUID',
                          `client_ip` VARCHAR(255) NOT NULL COMMENT '用户ip',
                          `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建日期',
                          `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新日期',
                          `screen_height` BIGINT DEFAULT NULL COMMENT '屏幕高度',
                          `screen_width` BIGINT DEFAULT NULL COMMENT '屏幕宽度',
                          `os` VARCHAR(255) DEFAULT NULL COMMENT '系统',
                          `os_version` VARCHAR(255) DEFAULT NULL COMMENT '系统版本',
                          `app_build_number` VARCHAR(255) DEFAULT NULL COMMENT 'app编译版本号',
                          `app_version` VARCHAR(255) DEFAULT NULL COMMENT 'app版本号',
                          `app_platform` VARCHAR(255) DEFAULT NULL COMMENT 'app平台：appstore或者googlepay',
                          `app_package_id` VARCHAR(255) DEFAULT NULL COMMENT 'app包名',
                          `device_brand` VARCHAR(255) DEFAULT NULL COMMENT '设备品牌',
                          `device_model` VARCHAR(255) DEFAULT NULL COMMENT '设备型号',
                          `radio` VARCHAR(255) DEFAULT NULL COMMENT '网络',
                          `carrier` VARCHAR(255) DEFAULT NULL COMMENT '运营商',
                          `is_wifi` BOOLEAN DEFAULT NULL COMMENT '是不是wifi',
                          `proxy` VARCHAR(255) DEFAULT NULL COMMENT '使用的代理',
                          `device_model_desc` VARCHAR(255) DEFAULT NULL COMMENT '设备名称',
                          `language_id` VARCHAR(255) DEFAULT NULL COMMENT '设备语言',
                          `is_del` INT NOT NULL DEFAULT 0 COMMENT '记录的删除标记',
                          `serial_number` VARCHAR(255) DEFAULT NULL COMMENT '设备序列号，仅macos有',
                          `platform_uuid` VARCHAR(255) DEFAULT NULL COMMENT '设备的硬件uuid，仅macos有',
                          PRIMARY KEY (`id`),
                          UNIQUE KEY `device_uuid_uindex` (`uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备信息表';

-- ----------------------------
-- 设备Token表
-- ----------------------------
DROP TABLE IF EXISTS `devicetoken`;
CREATE TABLE `devicetoken` (
                               `id` BIGINT NOT NULL COMMENT 'key ID',
                               `device_token` VARCHAR(255) NOT NULL COMMENT '推送服务的设备token',
                               `device_uuid` VARCHAR(255) NOT NULL COMMENT '设备的唯一id',
                               `platform` VARCHAR(255) NOT NULL COMMENT '平台',
                               `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录的创建时间，默认为当前时间',
                               `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录的更新时间，默认为当前时间',
                               `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '记录的删除时间',
                               `is_del` INT NOT NULL DEFAULT 0 COMMENT '记录的删除标记',
                               PRIMARY KEY (`id`),
                               UNIQUE KEY `devicetoken_device_uuid_uindex` (`device_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='device token 表';

-- ----------------------------
-- 用户设备关联表
-- ----------------------------
DROP TABLE IF EXISTS `userdevice`;
CREATE TABLE `userdevice` (
                              `id` BIGINT NOT NULL COMMENT '主键id',
                              `user_id` BIGINT NOT NULL COMMENT '用户id',
                              `device_token_id` BIGINT DEFAULT NULL COMMENT '关联的devicetoken的id',
                              `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录的创建时间，默认为当前时间',
                              `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录的更新时间，默认为当前时间',
                              `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '记录的删除时间',
                              `is_del` INT NOT NULL DEFAULT 0 COMMENT '记录的删除标记',
                              `device_id` BIGINT DEFAULT NULL COMMENT '关联的device的id',
                              `auth_key` VARCHAR(255) NOT NULL COMMENT '用于在端对端加密的加盐base64字符串',
                              `client_public_key` VARCHAR(255) NOT NULL COMMENT '客户端公钥',
                              `service_private_key` VARCHAR(255) NOT NULL COMMENT '服务端私钥',
                              `service_public_key` VARCHAR(255) NOT NULL COMMENT '服务端公钥',
                              `uuid` VARCHAR(255) NOT NULL COMMENT '设备id',
                              PRIMARY KEY (`id`),
                              KEY `userdevice_user_id_idx` (`user_id`),
                              CONSTRAINT `userdevice_user_id_fk` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION,
                              CONSTRAINT `userdevice_device_id_fk` FOREIGN KEY (`device_id`) REFERENCES `device` (`id`),
                              CONSTRAINT `userdevice_devicetoken_id_fk` FOREIGN KEY (`device_token_id`) REFERENCES `devicetoken` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关联用户设备的表';

-- ----------------------------
-- 用户反馈表
-- ----------------------------
DROP TABLE IF EXISTS `issues`;
CREATE TABLE `issues` (
                          `id` BIGINT NOT NULL COMMENT '主键id',
                          `user_id` BIGINT NOT NULL COMMENT '用户id',
                          `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
                          `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
                          `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间',
                          `status` VARCHAR(255) NOT NULL DEFAULT 'open' COMMENT '问题状态',
                          `type` VARCHAR(255) NOT NULL DEFAULT 'feedback' COMMENT '问题类型',
                          `sub_type` VARCHAR(255) DEFAULT NULL COMMENT '问题子类型',
                          `title` VARCHAR(255) NOT NULL COMMENT '问题标题',
                          `area` VARCHAR(255) DEFAULT NULL COMMENT '发生问题的位置，比如订阅或者聊天',
                          `body` JSON DEFAULT NULL COMMENT '问题描述',
                          `is_del` INT NOT NULL DEFAULT 0 COMMENT '记录的删除标记',
                          `version` VARCHAR(255) DEFAULT NULL COMMENT '版本号',
                          `platform` VARCHAR(255) DEFAULT NULL COMMENT '平台',
                          `language_name` VARCHAR(255) DEFAULT NULL COMMENT '语言',
                          PRIMARY KEY (`id`),
                          KEY `issues_user_id_idx` (`user_id`),
                          CONSTRAINT `issues_user_id_fk` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户反馈的问题';

SET FOREIGN_KEY_CHECKS = 1;

-- DROP TABLE IF EXISTS `userlog`;

CREATE TABLE `userlog` (
                           `id` BIGINT NOT NULL COMMENT '日志的主键id',
                           `user_id` BIGINT NULL COMMENT '产生日志的用户id',
                           `user_ip` VARCHAR(255) NOT NULL COMMENT '产生日志的用户ip',
                           `business` VARCHAR(255) NOT NULL COMMENT '商业',
                           `operation` VARCHAR(255) NOT NULL COMMENT '操作',
                           `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建日期',
                           `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新日期',
                           `screen_height` BIGINT NULL COMMENT '屏幕高度',
                           `screen_width` BIGINT NULL COMMENT '屏幕宽度',
                           `os` VARCHAR(255) NULL COMMENT '系统：iOS、android、web',
                           `os_version` VARCHAR(255) NULL COMMENT '系统版本',
                           `app_build_number` VARCHAR(255) NULL COMMENT 'app编译版本号',
                           `app_version` VARCHAR(255) NULL COMMENT 'app版本号',
                           `app_platform` VARCHAR(255) NULL COMMENT 'app平台：appstore或者googlepay',
                           `app_package_id` VARCHAR(255) NULL COMMENT 'app包名',
                           `device_brand` VARCHAR(255) NULL COMMENT '设备品牌',
                           `device_id` VARCHAR(255) NULL COMMENT '设备id',
                           `device_model` VARCHAR(255) NULL COMMENT '设备型号',
                           `radio` VARCHAR(255) NULL COMMENT '网络',
                           `carrier` VARCHAR(255) NULL COMMENT '运营商',
                           `is_wifi` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是不是wifi',
                           `proxy` VARCHAR(255) NULL COMMENT '使用的代理',
                           `device_model_desc` VARCHAR(255) NULL COMMENT '设备名称描述',
                           `language_id` VARCHAR(255) NULL COMMENT '设备的语言',
                           PRIMARY KEY (`id`),
                           INDEX `idx_userlog_user_id` (`user_id`),
                           CONSTRAINT `fk_userlog_user_id` FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户日志表';

-- DROP TABLE IF EXISTS `bill`;

CREATE TABLE `bill` (
                        `id` BIGINT NOT NULL COMMENT '账单ID',
                        `user_id` BIGINT NOT NULL COMMENT '用户ID',
                        `cost_change` DECIMAL(10, 2) NOT NULL COMMENT '变动金额',
                        `balance` DECIMAL(10, 2) NOT NULL COMMENT '账户余额',
                        `cost_comment` TEXT NOT NULL COMMENT '变动说明',
                        `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '记录的创建时间，默认为当前时间',
                        `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '记录的更新时间，默认为当前时间',
                        `cdkey_id` BIGINT NULL COMMENT '当使用cdk充值时，则为对应的cdk',
                        `extras` JSON NULL COMMENT '第三方订单信息，比如苹果内购的订单及校验信息',
                        `transaction_id` VARCHAR(255) NULL COMMENT '购买凭证，apple内购的事物id',
                        `original_transaction_id` VARCHAR(255) NULL COMMENT '苹果内购的原始id',
                        `subscribeproduct_id` BIGINT NULL COMMENT '订阅的id',
                        `bill_type` INT NULL COMMENT '订单的类型，消耗性和订阅类型',
                        `original_bill_id` BIGINT NULL COMMENT '原始账单id，退款时记录',
                        PRIMARY KEY (`id`),
                        INDEX `idx_bill_user_id` (`user_id`),
                        CONSTRAINT `fk_bill_user_id` FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON DELETE CASCADE
#                         CONSTRAINT `fk_bill_cdkey_id` FOREIGN KEY (`cdkey_id`) REFERENCES `cdkey`(`id`) ON DELETE CASCADE,
#                         CONSTRAINT `fk_bill_subscribeproduct_id` FOREIGN KEY (`subscribeproduct_id`) REFERENCES `subscribeproduct`(`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户账单表';

-- --------------------------------------------------------
-- Table structure for signals (信号指令表)
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS `signals` (
                                         `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '信号唯一ID (主键)',
                                         `symbol` VARCHAR(30) NOT NULL COMMENT '交易对 (e.g., BTCUSDT)',
                                         `command` VARCHAR(20) NOT NULL COMMENT '原始指令 (BUY/SELL)',
    -- 信号核心时间：K线收盘时间
                                         `timestamp` TIMESTAMP NOT NULL COMMENT '信号产生时间 (K线收盘)',

    -- 信号过期时间：风控边界
                                         `expiry_timestamp` TIMESTAMP NULL COMMENT '信号失效时间点 (策略最大持有周期)',

                                         `status` VARCHAR(10) NOT NULL COMMENT '信号状态 (ACTIVE, EXPIRED)',

                                         `final_score` DECIMAL(5,2) NOT NULL COMMENT '决策树过滤时的最终趋势分数',
                                         `explanation` TEXT COMMENT '决策树通过的原因/依据',

                                         `signal_period` VARCHAR(10) NOT NULL COMMENT '信号产生周期 (e.g., 5m, 15m)',
                                         `entry_price` DECIMAL(15,8) COMMENT '入场价格',
                                         `mark_price` DECIMAL(15,8) COMMENT 'k线收盘价格，也是计算信号的标价',
                                         `recommended_sl` DECIMAL(15,8) COMMENT '建议止损价格',
                                         `recommended_tp` DECIMAL(15,8) COMMENT '建议止盈价格',
                                         `chart_snapshot_url` VARCHAR(255) COMMENT 'K线图快照URL',
                                         `details_json` JSON COMMENT '存储 HighFreqIndicators 等复杂 JSON 结构',
                                         `is_premium` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否为精选信号，为付费用户提供',

                                         `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',

                                         PRIMARY KEY (`id`),
                                         INDEX `idx_symbol_status` (`symbol`, `status`),
                                         INDEX `idx_timestamp` (`timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易信号指令表';

-- --------------------------------------------------------
-- Table structure for trend_snapshots (趋势快照表)
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS `trend_snapshots` (
                                                 `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '趋势快照ID (主键)',

                                                 `signal_id` BIGINT UNSIGNED NOT NULL COMMENT '关联的信号ID (外键)',

                                                 `symbol` VARCHAR(30) NOT NULL COMMENT '交易对',

                                                 `timestamp` TIMESTAMP NOT NULL COMMENT '快照时间',
                                                 `direction` VARCHAR(10) NOT NULL COMMENT '趋势主方向',
                                                 `last_price` DECIMAL(15,8) NOT NULL COMMENT '信号发生时的价格',

                                                 `score_4h` DECIMAL(5,2) NOT NULL,
                                                 `score_1h` DECIMAL(5,2) NOT NULL,
                                                 `score_30m` DECIMAL(5,2) NOT NULL,
                                                 `atr` DECIMAL(15,8) NOT NULL,
                                                 `adx` DECIMAL(15,8) NOT NULL,
                                                 `rsi` DECIMAL(15,8) NOT NULL,

                                                 `final_score` DECIMAL(5,2) NOT NULL COMMENT '决策树过滤时的最终趋势分数',
                                                 `trend_score` DECIMAL(5,2) NOT NULL COMMENT '趋势分数',
                                                 `indicators_json` JSON COMMENT '所有周期的技术指标快照',

                                                 PRIMARY KEY (`id`),

    -- 确保一个信号只对应一个趋势快照 (一对一关系的关键)
                                                 UNIQUE KEY `uk_signal_id` (`signal_id`),

    -- 建立外键约束
                                                 CONSTRAINT `fk_signal_id` FOREIGN KEY (`signal_id`)
                                                     REFERENCES `signals` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='趋势快照和指标数据表';

-- --------------------------------------------------------
-- 信号模拟结果表 (signal_outcomes) DDL
-- --------------------------------------------------------
-- 用于存储每个信号在模拟回测下的最终结果，用于计算历史胜率。
CREATE TABLE IF NOT EXISTS `signal_outcomes` (
                                                 `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '结果唯一ID (主键)',

    -- 关联 signals 表的主键
                                                 `signal_id` BIGINT UNSIGNED NOT NULL UNIQUE COMMENT '关联的信号ID (外键)',

                                                 `symbol` VARCHAR(30) NOT NULL COMMENT '交易对',

                                                 `outcome` VARCHAR(10) NOT NULL COMMENT '模拟结果 (HIT_TP, HIT_SL, EXPIRED)',

    -- 存储 PnL 百分比，例如 4% 盈利记为 0.04，2% 亏损记为 -0.02
                                                 `final_pnl_pct` DECIMAL(10,4) NOT NULL COMMENT '最终盈亏百分比',

                                                 `candles_used` INT UNSIGNED NOT NULL COMMENT '达到结果所消耗的K线数量',
                                                 `closed_at` TIMESTAMP NOT NULL COMMENT '平仓时间 (即触及TP/SL或过期的时间)',

                                                 `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

                                                 PRIMARY KEY (`id`),
                                                 UNIQUE KEY `uk_signal_id_outcome` (`signal_id`),
    -- 外键约束:
                                                FOREIGN KEY (`signal_id`) REFERENCES `signals`(`id`),
                                                 INDEX `idx_symbol_outcome` (`symbol`, `outcome`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='信号模拟结果表';

CREATE TABLE `alert_subscription` (
    -- 主键
                                      `id` VARCHAR(36) NOT NULL PRIMARY KEY,

    -- 索引字段
                                      `user_id` VARCHAR(36) NOT NULL,
                                      `inst_id` VARCHAR(30) NOT NULL,

    -- 提醒规则
                                      `alert_type` INT NOT NULL COMMENT '消息类型 (1=PRICE, 2=STRATEGY, 4=LISTING等)',
                                      `direction` VARCHAR(10) NOT NULL COMMENT 'UP/DOWN/RATE',

    -- 价格突破字段 (使用 DECIMAL 保证精度)
                                      `target_price` DECIMAL(20, 8) NULL COMMENT '目标价格',

    -- 极速提醒字段
                                      `change_percent` DECIMAL(6, 2) NULL COMMENT '变化百分比',
                                      `window_minutes` INT NULL COMMENT '时间窗口 (分钟)',

    -- 状态字段
                                      `is_active` BOOLEAN NOT NULL COMMENT '是否活跃 (1=待触发, 0=已触发)',
                                      `last_triggered_price` DECIMAL(20, 8) NULL COMMENT '上次触发时的价格，用于重置判断',
                                      `boundary_precision` DECIMAL(10, 8) NULL COMMENT '0.01 表示以 0.01 为单位跨越',

    -- Gorm 约定时间戳
                                      `created_at` DATETIME NOT NULL,
                                      `updated_at` DATETIME NOT NULL,

    -- 联合索引：方便根据用户和币种查找订阅
                                      INDEX `idx_user_inst` (`user_id`, `inst_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户提醒订阅规则表';


CREATE TABLE `alert_history` (
    -- 主键
                                 `id` VARCHAR(36) NOT NULL PRIMARY KEY,

    -- 索引字段
                                 `user_id` VARCHAR(36) NOT NULL COMMENT '接收者用户ID',

    -- 消息内容和类型
                                 `subscription_id` VARCHAR(36) NULL COMMENT '关联的订阅ID',
                                 `title` VARCHAR(100) NOT NULL,
                                 `content` TEXT NOT NULL,
                                 `level` INT NOT NULL COMMENT '消息级别 (1=WARNING, 2=CRITICAL)',
                                 `alert_type` INT NOT NULL COMMENT '消息类型',

    -- 时间戳
                                 `timestamp` BIGINT NOT NULL COMMENT '消息时间戳 (毫秒)',

    -- 可扩展字段，使用 JSON 存储 Protobuf 的 extra map
                                 `extra_json` JSON NULL COMMENT '额外数据，JSON格式',

    -- Gorm 约定时间戳
                                 `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 联合索引：方便根据用户ID和时间倒序查询历史记录
                                 INDEX `idx_user_ts` (`user_id`, `timestamp` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='已发送的提醒消息历史记录表';