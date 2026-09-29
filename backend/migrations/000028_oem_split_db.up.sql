-- 000028 OEM 分销独立成库
-- 背景：OEM 分销相关表（oem / price_override / oem_settlement）原先落在系统库 oem_sys，
--       与平台账户/账单等核心表混放。本迁移将其迁至独立库 starloft_oem，并补建库存出库流水表
--       oem_stock_log（资源包扣量不入 bill，故单独记流水以闭合「进货 − 出库 = 结存」对账）。
-- 说明：
--   1) user.oem_id 属于用户归属字段，仍留在 oem_sys.user（不随表迁走）。
--      跨库 JOIN 在同一 MySQL 实例内可用，故下级用户 + 结算汇总查询不受影响。
--   2) setting 与产品库 product_config 属配置体系改造，与 OEM 无关，保持在原库。
--   3) 迁移整体幂等：新库/新表不存在才创建；存量数据仅在旧表存在且新表为空时迁移一次。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1. 建库
CREATE DATABASE IF NOT EXISTS `starloft_oem` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

-- 2. 新库建表（与系统库原结构一致）

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem` (
        `id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（与用户表主键一致）'',
        `name` varchar(100) NOT NULL DEFAULT '''' COMMENT ''OEM 品牌名'',
        `domain` varchar(190) NOT NULL DEFAULT '''' COMMENT ''自有域名（空表示未配置）'',
        `contact` varchar(100) NOT NULL DEFAULT '''' COMMENT ''联系方式'',
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

-- 3. 补建库存出库流水表（新增能力，无存量数据）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_stock_log');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `starloft_oem`.`oem_stock_log` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（出库方）'',
        `user_id` bigint NOT NULL COMMENT ''消耗用户 user_id'',
        `product` varchar(32) NOT NULL DEFAULT '''' COMMENT ''服务标识（fv_auth/fv_self/sms）'',
        `source` varchar(16) NOT NULL DEFAULT ''pack'' COMMENT ''pack-资源包出库 balance-余额补齐'',
        `pack_id` bigint NOT NULL DEFAULT 0 COMMENT ''出库的用户资源包 ID（余额补齐为 0）'',
        `count` int NOT NULL DEFAULT 0 COMMENT ''本次出库条数/次数'',
        `balance_amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''余额补齐金额（资源包出库为 0）'',
        `ref_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''关联业务表（auth_record/sms_send_record/user_resource_pack）'',
        `ref_id` bigint NOT NULL DEFAULT 0 COMMENT ''关联业务主键'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''备注'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_stock_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_stock_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 库存出库流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. 存量数据迁移（旧表存在 且 新表为空 时迁移一次）

SET @old_oem = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'oem');
SET @new_oem_rows = (SELECT COUNT(*) FROM `starloft_oem`.`oem`);
SET @sql = IF(@old_oem > 0 AND @new_oem_rows = 0,
    'INSERT INTO `starloft_oem`.`oem` (id, name, domain, contact, status, reject_reason, remark, created_at, updated_at)
     SELECT id, name, domain, contact, status, reject_reason, remark, created_at, updated_at FROM `oem_sys`.`oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @old_po = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @new_po_rows = (SELECT COUNT(*) FROM `starloft_oem`.`price_override`);
SET @sql = IF(@old_po > 0 AND @new_po_rows = 0,
    'INSERT INTO `starloft_oem`.`price_override` (id, scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT id, scope_type, scope_id, price_type, target, price, created_at, updated_at FROM `oem_sys`.`price_override`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @old_se = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'oem_settlement');
SET @new_se_rows = (SELECT COUNT(*) FROM `starloft_oem`.`oem_settlement`);
SET @sql = IF(@old_se > 0 AND @new_se_rows = 0,
    'INSERT INTO `starloft_oem`.`oem_settlement` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
     SELECT id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at FROM `oem_sys`.`oem_settlement`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 5. 清理系统库旧表（数据已迁至 starloft_oem）
DROP TABLE IF EXISTS `oem_sys`.`oem_settlement`;
DROP TABLE IF EXISTS `oem_sys`.`price_override`;
DROP TABLE IF EXISTS `oem_sys`.`oem`;