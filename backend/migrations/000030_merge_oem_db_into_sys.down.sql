-- 000030 回滚：把推广表从系统库迁回独立库 starloft_oem，并恢复 OEM 语义表名
-- 说明：被丢弃的六张表（账户/等级/库存）本次仅重建空结构，历史数据不可恢复。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- ============================================================
-- 1. 重建独立库
-- ============================================================
CREATE DATABASE IF NOT EXISTS `starloft_oem` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

-- 1.1 oem（推广商主体还原）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem` (
        `id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（与用户表主键一致）'',
        `name` varchar(100) NOT NULL DEFAULT '''' COMMENT ''OEM 品牌名'',
        `domain` varchar(190) NOT NULL DEFAULT '''' COMMENT ''自有域名（空表示未配置）'',
        `contact` varchar(100) NOT NULL DEFAULT '''' COMMENT ''联系方式'',
        `commission_rate` decimal(6,4) NOT NULL DEFAULT 0.2 COMMENT ''提成比例（0~1）'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待审核 1-已通过 2-已驳回 3-已停用'',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''' COMMENT ''驳回原因'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''平台备注'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_domain` (`domain`),
        KEY `idx_oem_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 分销主体''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.2 price_override
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''user-指定用户 oem-OEM 给下级定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id；=oem 时为 OEM 管理员 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识；pack 时为「产品标识:资源包ID」'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''价格覆盖（OEM/单用户）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.3 oem_settlement
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_settlement');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_settlement` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（收款方）'',
        `user_id` bigint NOT NULL COMMENT ''结算来源用户 user_id（付款方）'',
        `amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''结算金额，正=结算给 OEM，负=退款冲回'',
        `biz_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''pack_purchase/sms_send/fv_auth/refund'',
        `ref_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''关联业务表类型'',
        `ref_id` bigint NOT NULL DEFAULT 0 COMMENT ''关联业务主键'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''备注'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_settle_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_settle_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 结算流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.4 oem_withdraw
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_withdraw');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_withdraw` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL,
        `amount` decimal(12,2) NOT NULL,
        `fee_rate` decimal(6,4) NOT NULL DEFAULT 0.01,
        `fee` decimal(12,2) NOT NULL DEFAULT 0,
        `actual_amount` decimal(12,2) NOT NULL DEFAULT 0,
        `status` tinyint NOT NULL DEFAULT 0,
        `payee_info` varchar(500) NOT NULL DEFAULT '''',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''',
        `admin_id` bigint NOT NULL DEFAULT 0,
        `reviewed_at` datetime(3) NULL,
        `paid_at` datetime(3) NULL,
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_withdraw_oem` (`oem_id`),
        KEY `idx_oem_withdraw_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 提现申请''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.5 重建已丢弃表的空结构：oem_account（三口径账户）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_account');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_account` (
        `oem_id` bigint NOT NULL,
        `balance` decimal(12,2) NOT NULL DEFAULT 0,
        `frozen` decimal(12,2) NOT NULL DEFAULT 0,
        `available` decimal(12,2) NOT NULL DEFAULT 0,
        `total_recharge` decimal(12,2) NOT NULL DEFAULT 0,
        `level_id` bigint NOT NULL DEFAULT 0,
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`oem_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 财务账户''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.6 oem_account_log（三口径财务流水）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_account_log');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_account_log` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL,
        `user_id` bigint NOT NULL DEFAULT 0,
        `biz_type` varchar(32) NOT NULL DEFAULT '''',
        `balance_change` decimal(12,2) NOT NULL DEFAULT 0,
        `frozen_change` decimal(12,2) NOT NULL DEFAULT 0,
        `available_change` decimal(12,2) NOT NULL DEFAULT 0,
        `balance_after` decimal(12,2) NOT NULL DEFAULT 0,
        `frozen_after` decimal(12,2) NOT NULL DEFAULT 0,
        `available_after` decimal(12,2) NOT NULL DEFAULT 0,
        `product` varchar(32) NOT NULL DEFAULT '''',
        `service` varchar(32) NOT NULL DEFAULT '''',
        `ref_type` varchar(32) NOT NULL DEFAULT '''',
        `ref_id` bigint NOT NULL DEFAULT 0,
        `remark` varchar(255) NOT NULL DEFAULT '''',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_fund_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_fund_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 财务流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.7 oem_user_account（下级用户子账户）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_user_account');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_user_account` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL,
        `user_id` bigint NOT NULL,
        `balance` decimal(12,2) NOT NULL DEFAULT 0,
        `total_recharge` decimal(12,2) NOT NULL DEFAULT 0,
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_oem_user_account` (`oem_id`,`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''下级用户在 OEM 下的子账户''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.8 oem_level / oem_level_price（进货等级与等级成本价）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_level');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_level` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `code` varchar(16) NOT NULL,
        `name` varchar(50) NOT NULL DEFAULT '''',
        `threshold` decimal(12,2) NOT NULL DEFAULT 0,
        `sort_order` int NOT NULL DEFAULT 0,
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_oem_level_code` (`code`),
        KEY `idx_oem_level_sort` (`sort_order`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 等级''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_level_price');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_level_price` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `level_id` bigint NOT NULL,
        `service` varchar(32) NOT NULL,
        `price` decimal(10,4) NOT NULL DEFAULT 0,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_oem_level_service` (`level_id`,`service`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''等级服务成本价''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.9 oem_stock_log（库存出库流水）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_stock_log');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_stock_log` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL,
        `user_id` bigint NOT NULL,
        `product` varchar(32) NOT NULL DEFAULT '''',
        `source` varchar(16) NOT NULL DEFAULT ''pack'',
        `pack_id` bigint NOT NULL DEFAULT 0,
        `count` int NOT NULL DEFAULT 0,
        `balance_amount` decimal(10,2) NOT NULL DEFAULT 0,
        `ref_type` varchar(32) NOT NULL DEFAULT '''',
        `ref_id` bigint NOT NULL DEFAULT 0,
        `remark` varchar(255) NOT NULL DEFAULT '''',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_stock_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_stock_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 库存出库流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 2. 数据迁回独立库（新库表存在 且 目标表为空 时迁移一次）
-- ============================================================
SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @dst_rows = (SELECT COUNT(*) FROM `starloft_oem`.`oem`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `starloft_oem`.`oem` (id, name, domain, contact, status, reject_reason, remark, created_at, updated_at)
     SELECT id, name, domain, contact, status, reject_reason, remark, created_at, updated_at FROM `oem_sys`.`affiliate`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @dst_rows = (SELECT COUNT(*) FROM `starloft_oem`.`price_override`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `starloft_oem`.`price_override` (id, scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT id, scope_type, scope_id, price_type, target, price, created_at, updated_at FROM `oem_sys`.`price_override`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_commission');
SET @dst_rows = (SELECT COUNT(*) FROM `starloft_oem`.`oem_settlement`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `starloft_oem`.`oem_settlement` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
     SELECT id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at FROM `oem_sys`.`aff_commission`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @dst_rows = (SELECT COUNT(*) FROM `starloft_oem`.`oem_withdraw`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `starloft_oem`.`oem_withdraw` (id, oem_id, amount, fee_rate, fee, actual_amount, status, payee_info, reject_reason, admin_id, reviewed_at, paid_at, created_at, updated_at)
     SELECT id, oem_id, amount, fee_rate, fee, actual_amount, status, payee_info, reject_reason, admin_id, reviewed_at, paid_at, created_at, updated_at FROM `oem_sys`.`aff_withdraw`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 3. 删除系统库中的推广语义新表
-- ============================================================
DROP TABLE IF EXISTS `oem_sys`.`aff_withdraw`;
DROP TABLE IF EXISTS `oem_sys`.`aff_commission`;
DROP TABLE IF EXISTS `oem_sys`.`price_override`;
DROP TABLE IF EXISTS `oem_sys`.`affiliate`;
