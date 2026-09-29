-- 000047 拆除推广商主体表 affiliate：推广码落到用户/员工账号，归属与提成改为 referrer_type + referrer_id
-- 背景：推广不再审核——用户打开推广页即开通（推广码自动生成），不需要推广商主体表。
--       员工销售保留：推广码迁到 admin_user.aff_code；提成与提现的归属列由 affiliate.id（oem_id）
--       改为 referrer_type + referrer_id（user→user.id，staff→admin_user.id）。
-- 处理：
--   ① user 加 aff_code（可空+唯一）、referrer_type、referrer_id，按 affiliate 回填归属（status=1 才回填），
--      再删 oem_id 与其索引；
--   ② admin_user 加 aff_code（可空+唯一），按 affiliate 中 referrer_type='staff' 的行回填；
--   ③ user_commission / staff_commission / aff_withdraw 加 referrer_type + referrer_id，按 affiliate 回填，
--      再删 oem_id 与其索引，补 referrer 复合索引；
--   ④ 最后 DROP TABLE affiliate。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。整份迁移幂等（列/索引/表存在性判断）。
--       全新库表结构由 AutoMigrate 补齐（迁移先于 AutoMigrate 执行），故所有 ADD 操作在表不存在时跳过。
--       存量 affiliate.status∈{0,2,3} 的行不参与回填，其下级归属清空（回落平台直营）。

-- ============================================================
-- ① user：推广码 + 归属列
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user` ADD COLUMN `aff_code` varchar(12) NULL COMMENT ''推广码（12 位数字+小写字母，NULL=未开通推广）'' AFTER `status`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'referrer_type');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user` ADD COLUMN `referrer_type` varchar(8) NOT NULL DEFAULT '''' COMMENT ''归属推介方类型：user-用户 staff-员工（空=平台直营）'' AFTER `aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'referrer_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user` ADD COLUMN `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''归属推介方 ID（user→user.id，staff→admin_user.id；0=平台直营）'' AFTER `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 按 affiliate 回填归属：仅已通过审核（status=1）的推广商才回填，其余留空=平台直营
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_oem_id > 0,
    'UPDATE `oem_sys`.`user` u JOIN `oem_sys`.`affiliate` a ON a.id = u.oem_id AND a.status = 1
        SET u.referrer_type = a.referrer_type, u.referrer_id = a.referrer_id
      WHERE u.oem_id > 0 AND u.referrer_id = 0',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 推广码唯一键（可空唯一：允许多个 NULL）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'uk_user_aff_code');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user` ADD UNIQUE KEY `uk_user_aff_code` (`aff_code`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 归属查询索引
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_user_referrer');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user` ADD KEY `idx_user_referrer` (`referrer_type`, `referrer_id`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 删除旧归属列 oem_id 与其索引
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_user_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user` DROP INDEX `idx_user_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_oem_id > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ② admin_user：员工推广码
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`admin_user` ADD COLUMN `aff_code` varchar(12) NULL COMMENT ''员工推广码（12 位数字+小写字母，NULL=未生成）'' AFTER `status`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 按 affiliate 中 referrer_type='staff' 的行回填员工推广码
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_tbl > 0 AND @has_col > 0,
    'UPDATE `oem_sys`.`admin_user` ad JOIN `oem_sys`.`affiliate` a
        ON a.referrer_type = ''staff'' AND a.referrer_id = ad.id
        SET ad.aff_code = a.aff_code
      WHERE a.aff_code IS NOT NULL AND a.aff_code <> '''' AND ad.aff_code IS NULL',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND INDEX_NAME = 'uk_admin_aff_code');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`admin_user` ADD UNIQUE KEY `uk_admin_aff_code` (`aff_code`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ③ 提成流水 / 提现：oem_id → referrer_type + referrer_id
-- ============================================================

-- ③.1 user_commission
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND COLUMN_NAME = 'referrer_type');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user_commission` ADD COLUMN `referrer_type` varchar(8) NOT NULL DEFAULT '''' COMMENT ''推介方类型：user-用户 staff-员工'' AFTER `id`, ADD COLUMN `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''推介方 ID（收款方）'' AFTER `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_oem_id > 0,
    'UPDATE `oem_sys`.`user_commission` c JOIN `oem_sys`.`affiliate` a ON a.id = c.oem_id
        SET c.referrer_type = a.referrer_type, c.referrer_id = a.referrer_id
      WHERE c.oem_id > 0',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` DROP INDEX `idx_user_settle_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_oem_id > 0,
    'ALTER TABLE `oem_sys`.`user_commission` DROP COLUMN `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_referrer');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user_commission` ADD KEY `idx_user_settle_referrer` (`referrer_type`, `referrer_id`, `created_at`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ③.2 staff_commission
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND COLUMN_NAME = 'referrer_type');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`staff_commission` ADD COLUMN `referrer_type` varchar(8) NOT NULL DEFAULT '''' COMMENT ''推介方类型（恒为 staff）'' AFTER `id`, ADD COLUMN `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''员工销售 admin_user.id（收款方）'' AFTER `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_oem_id > 0,
    'UPDATE `oem_sys`.`staff_commission` c JOIN `oem_sys`.`affiliate` a ON a.id = c.oem_id
        SET c.referrer_type = a.referrer_type, c.referrer_id = a.referrer_id
      WHERE c.oem_id > 0',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND INDEX_NAME = 'idx_staff_settle_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`staff_commission` DROP INDEX `idx_staff_settle_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_oem_id > 0,
    'ALTER TABLE `oem_sys`.`staff_commission` DROP COLUMN `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission' AND INDEX_NAME = 'idx_staff_settle_referrer');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`staff_commission` ADD KEY `idx_staff_settle_referrer` (`referrer_type`, `referrer_id`, `created_at`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ③.3 aff_withdraw
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'referrer_type');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` ADD COLUMN `referrer_type` varchar(8) NOT NULL DEFAULT ''user'' COMMENT ''推介方类型（提现仅用户型推广）'' AFTER `id`, ADD COLUMN `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''用户型推广者 user.id'' AFTER `referrer_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_oem_id > 0,
    'UPDATE `oem_sys`.`aff_withdraw` w JOIN `oem_sys`.`affiliate` a ON a.id = w.oem_id
        SET w.referrer_type = a.referrer_type, w.referrer_id = a.referrer_id
      WHERE w.oem_id > 0',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND INDEX_NAME = 'idx_aff_withdraw_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` DROP INDEX `idx_aff_withdraw_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_oem_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_oem_id > 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` DROP COLUMN `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND INDEX_NAME = 'idx_aff_withdraw_referrer');
SET @sql = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` ADD KEY `idx_aff_withdraw_referrer` (`referrer_type`, `referrer_id`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- ④ 拆除推广商主体表（无外键，删表不被 DB 阻塞）
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate');
SET @sql = IF(@has_tbl > 0,
    'DROP TABLE `oem_sys`.`affiliate`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
