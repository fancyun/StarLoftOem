-- 000030 合并 starloft_oem 库到 oem_sys（OEM 分销改造为「推广分佣」）
-- 背景：推广模式不再有「进货余额 / 等级成本价 / 库存出库」等概念，独立库失去存在意义。
-- 本迁移：
--   1) 保留四张表并并入系统库，同时改为推广语义表名：
--        starloft_oem.oem            → oem_sys.affiliate      （推广商主体）
--        starloft_oem.price_override → oem_sys.price_override （价格覆盖，仅保留 scope_type='user'）
--        starloft_oem.oem_settlement → oem_sys.aff_commission （提成流水）
--        starloft_oem.oem_withdraw   → oem_sys.aff_withdraw   （推广提现）
--   2) 丢弃六张废表（业务已移除，不再重建）：
--        oem_account / oem_account_log / oem_user_account / oem_level / oem_level_price / oem_stock_log
--   3) 只迁移 scope_type='user' 的价格覆盖（推广商自主定价能力取消）
--   4) 最后 DROP DATABASE starloft_oem
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
-- 说明：整份迁移幂等——建表前判存在，迁数据前判新表为空，DROP 一律 IF EXISTS。

-- ============================================================
-- 1. 系统库建表（结构与原表一致，仅表名改为推广语义）
-- ============================================================

-- 1.1 推广商主体（原 oem）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`affiliate` (
        `id` bigint NOT NULL COMMENT ''推广商 user_id（与用户表主键一致）'',
        `name` varchar(100) NOT NULL DEFAULT '''' COMMENT ''推广商名称'',
        `domain` varchar(190) NOT NULL DEFAULT '''' COMMENT ''自有域名（空表示未配置）'',
        `contact` varchar(100) NOT NULL DEFAULT '''' COMMENT ''联系方式'',
        `commission_rate` decimal(6,4) NOT NULL DEFAULT 0.2 COMMENT ''提成比例（0~1）'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待审核 1-已通过 2-已驳回 3-已停用'',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''' COMMENT ''驳回原因'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''平台备注'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_affiliate_domain` (`domain`),
        KEY `idx_affiliate_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''推广商主体''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.2 价格覆盖（单用户定向定价，平台级能力，表名不变）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''固定为 user-指定用户定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识；pack 时为「产品标识:资源包ID」'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''价格覆盖（单用户）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.3 提成流水（原 oem_settlement）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_commission');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`aff_commission` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''推广商 user_id（收款方）'',
        `user_id` bigint NOT NULL COMMENT ''提成来源用户 user_id'',
        `amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''提成金额，正=计提，负=退款冲回'',
        `biz_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''pack_purchase/sms_send/fv_auth/refund'',
        `ref_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''关联业务表类型'',
        `ref_id` bigint NOT NULL DEFAULT 0 COMMENT ''关联业务主键'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''备注'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_settle_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_settle_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''推广提成流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 1.4 推广提现（原 oem_withdraw）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`aff_withdraw` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''推广商 user_id'',
        `amount` decimal(12,2) NOT NULL COMMENT ''申请金额'',
        `fee_rate` decimal(6,4) NOT NULL DEFAULT 0.01 COMMENT ''手续费率'',
        `fee` decimal(12,2) NOT NULL DEFAULT 0 COMMENT ''手续费'',
        `actual_amount` decimal(12,2) NOT NULL DEFAULT 0 COMMENT ''实际打款额（申请额 − 手续费）'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待审核 1-已通过 2-已完成 3-已驳回'',
        `payee_info` varchar(500) NOT NULL DEFAULT '''' COMMENT ''收款信息（户名 / 方式 / 账号）'',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''' COMMENT ''驳回原因'',
        `admin_id` bigint NOT NULL DEFAULT 0 COMMENT ''处理管理员 id'',
        `reviewed_at` datetime(3) NULL,
        `paid_at` datetime(3) NULL,
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_aff_withdraw_oem` (`oem_id`),
        KEY `idx_aff_withdraw_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''推广提现申请''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 2. 存量数据迁移（旧表存在 且 新表为空 时迁移一次）
-- ============================================================

-- 2.1 oem → affiliate
SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem');
SET @dst_rows = (SELECT COUNT(*) FROM `oem_sys`.`affiliate`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `oem_sys`.`affiliate` (id, name, domain, contact, status, reject_reason, remark, created_at, updated_at)
     SELECT id, name, domain, contact, status, reject_reason, remark, created_at, updated_at FROM `starloft_oem`.`oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2.2 price_override → price_override（仅单用户定价，丢弃 scope_type=''oem'' 的推广商自主定价）
SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'price_override');
SET @dst_rows = (SELECT COUNT(*) FROM `oem_sys`.`price_override`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `oem_sys`.`price_override` (id, scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT id, scope_type, scope_id, price_type, target, price, created_at, updated_at
       FROM `starloft_oem`.`price_override` WHERE scope_type = ''user''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2.3 oem_settlement → aff_commission
SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_settlement');
SET @dst_rows = (SELECT COUNT(*) FROM `oem_sys`.`aff_commission`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `oem_sys`.`aff_commission` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
     SELECT id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at FROM `starloft_oem`.`oem_settlement`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2.4 oem_withdraw → aff_withdraw
SET @src_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_withdraw');
SET @dst_rows = (SELECT COUNT(*) FROM `oem_sys`.`aff_withdraw`);
SET @sql = IF(@src_tbl > 0 AND @dst_rows = 0,
    'INSERT INTO `oem_sys`.`aff_withdraw` (id, oem_id, amount, fee_rate, fee, actual_amount, status, payee_info, reject_reason, admin_id, reviewed_at, paid_at, created_at, updated_at)
     SELECT id, oem_id, amount, fee_rate, fee, actual_amount, status, payee_info, reject_reason, admin_id, reviewed_at, paid_at, created_at, updated_at FROM `starloft_oem`.`oem_withdraw`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 兜底：清理系统库中可能残留的推广商自主定价记录（能力已取消）
DELETE FROM `oem_sys`.`price_override` WHERE scope_type = 'oem';

-- ============================================================
-- 3. 丢弃废表（含新增的 oem_stock_log 与 AutoMigrate 建的账户/等级表）
-- ============================================================
DROP TABLE IF EXISTS `starloft_oem`.`oem_stock_log`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_account_log`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_account`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_user_account`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_level_price`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_level`;

-- 清理旧库中已迁走的四张表，随后删库
DROP TABLE IF EXISTS `starloft_oem`.`oem_settlement`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_withdraw`;
DROP TABLE IF EXISTS `starloft_oem`.`price_override`;
DROP TABLE IF EXISTS `starloft_oem`.`oem`;
DROP DATABASE IF EXISTS `starloft_oem`;

-- 系统库中历史遗留的同名旧表（000028 已删除，此处幂等兜底）
DROP TABLE IF EXISTS `oem_sys`.`oem_settlement`;
DROP TABLE IF EXISTS `oem_sys`.`oem`;
