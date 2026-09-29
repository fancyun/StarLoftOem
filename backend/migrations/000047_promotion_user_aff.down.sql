-- 000047 回滚：恢复 affiliate 表结构与 oem_id 归属列
-- 说明（重要）：推广商主体表 affiliate 被 000047 删除后，其行数据与「affiliate.id → oem_id」的映射不可恢复，
--       因此本回滚仅能重建表结构与旧列，无法还原 affiliate 行数据，也无法把 referrer_type/referrer_id 映射回 oem_id
--       （重建的 affiliate 为空表，回填后 oem_id 一律为 0）。升级前请务必备份数据库。
-- 处理（与 up 逆序）：
--   ① 重建 affiliate 表（结构同 000042 时的最终形态，空表）；
--   ② user_commission / staff_commission / aff_withdraw 加回 oem_id 与旧索引，删 referrer_* 列与新索引；
--   ③ admin_user 删 aff_code 与其索引；
--   ④ user 加回 oem_id 与 idx_user_oem，删 aff_code / referrer_* 列与其索引。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。整份迁移幂等（列/索引/表存在性判断）。

-- ============================================================
-- ① 重建 affiliate 表（空表）
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`affiliate` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `referrer_type` varchar(8) NOT NULL DEFAULT ''user'' COMMENT ''推介方类型：user-用户 staff-员工'',
        `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''推介方 ID（user 类型为 user.id，staff 类型为 admin_user.id）'',
        `name` varchar(100) NOT NULL DEFAULT '''' COMMENT ''推广商名称'',
        `domain` varchar(190) NOT NULL DEFAULT '''' COMMENT ''自有域名（空表示未配置）'',
        `aff_code` varchar(12) NULL COMMENT ''推广码（12 位数字+小写字母）'',
        `contact` varchar(100) NOT NULL DEFAULT '''' COMMENT ''联系方式'',
        `commission_rate` decimal(6,4) NOT NULL DEFAULT 0.2 COMMENT ''提成比例（0~1）'',
        `commission_rate_fv` decimal(6,4) NOT NULL DEFAULT 0.2 COMMENT ''人脸核验（fv）提成比例（0~1）'',
        `commission_rate_sms` decimal(6,4) NOT NULL DEFAULT 0.2 COMMENT ''短信（sms）提成比例（0~1）'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待审核 1-已通过 2-已驳回 3-已停用'',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''' COMMENT ''驳回原因'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''平台备注'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_affiliate_referrer` (`referrer_type`, `referrer_id`),
        UNIQUE KEY `uk_affiliate_code` (`aff_code`),
        KEY `idx_affiliate_domain` (`domain`),
        KEY `idx_affiliate_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''推广商主体''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ② 提成流水 / 提现：加回 oem_id（空表 affiliate，回填恒为 0），删 referrer_*
-- ============================================================

-- ②.1 user_commission
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user_commission` ADD COLUMN `oem_id` bigint NOT NULL DEFAULT 0 COMMENT ''推广商 affiliate.id（收款方）'' AFTER `id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_oem');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user_commission` ADD KEY `idx_user_settle_oem` (`oem_id`, `created_at`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_referrer');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` DROP INDEX `idx_user_settle_referrer`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND COLUMN_NAME = 'referrer_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user_commission` DROP COLUMN `referrer_id`, DROP COLUMN `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ②.2 staff_commission
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`staff_commission` ADD COLUMN `oem_id` bigint NOT NULL DEFAULT 0 COMMENT ''员工销售 affiliate.id（收款方）'' AFTER `id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND INDEX_NAME = 'idx_staff_settle_oem');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`staff_commission` ADD KEY `idx_staff_settle_oem` (`oem_id`, `created_at`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND INDEX_NAME = 'idx_staff_settle_referrer');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`staff_commission` DROP INDEX `idx_staff_settle_referrer`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND COLUMN_NAME = 'referrer_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`staff_commission` DROP COLUMN `referrer_id`, DROP COLUMN `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ②.3 aff_withdraw
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` ADD COLUMN `oem_id` bigint NOT NULL DEFAULT 0 COMMENT ''推广商 user_id'' AFTER `id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND INDEX_NAME = 'idx_aff_withdraw_oem');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` ADD KEY `idx_aff_withdraw_oem` (`oem_id`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND INDEX_NAME = 'idx_aff_withdraw_referrer');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` DROP INDEX `idx_aff_withdraw_referrer`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'referrer_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` DROP COLUMN `referrer_id`, DROP COLUMN `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ③ admin_user：删 aff_code
-- ============================================================
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND INDEX_NAME = 'uk_admin_aff_code');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`admin_user` DROP INDEX `uk_admin_aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`admin_user` DROP COLUMN `aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ④ user：加回 oem_id，删 aff_code / referrer_*
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user` ADD COLUMN `oem_id` bigint NOT NULL DEFAULT 0 COMMENT ''所属 OEM（=直接上级管理员 user_id，0=平台直营）'' AFTER `status`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_user_oem');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user` ADD KEY `idx_user_oem` (`oem_id`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_user_referrer');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user` DROP INDEX `idx_user_referrer`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'uk_user_aff_code');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user` DROP INDEX `uk_user_aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'referrer_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `referrer_id`, DROP COLUMN `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
