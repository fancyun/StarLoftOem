-- 000042 回滚：移除 affiliate 的推介方字段并取消 id 自增
-- 注意：已由自增生成的新 id 不会回退；回滚后必须同步回滚代码（旧代码按 userID 直查 affiliate.id）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等。

-- 删除唯一键
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND INDEX_NAME = 'uk_affiliate_referrer');
SET @sql_drop_idx = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP INDEX `uk_affiliate_referrer`',
    'SELECT 1');
PREPARE stmt_drop_idx FROM @sql_drop_idx; EXECUTE stmt_drop_idx; DEALLOCATE PREPARE stmt_drop_idx;

-- 取消 id 自增
SET @is_auto = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'id'
      AND EXTRA LIKE '%auto_increment%');
SET @sql_noauto = IF(@is_auto > 0,
    'ALTER TABLE `oem_sys`.`affiliate` MODIFY COLUMN `id` bigint NOT NULL',
    'SELECT 1');
PREPARE stmt_noauto FROM @sql_noauto; EXECUTE stmt_noauto; DEALLOCATE PREPARE stmt_noauto;

-- 删除 referrer_id
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'referrer_id');
SET @sql_drop_id = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP COLUMN `referrer_id`',
    'SELECT 1');
PREPARE stmt_drop_id FROM @sql_drop_id; EXECUTE stmt_drop_id; DEALLOCATE PREPARE stmt_drop_id;

-- 删除 referrer_type
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'referrer_type');
SET @sql_drop_type = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP COLUMN `referrer_type`',
    'SELECT 1');
PREPARE stmt_drop_type FROM @sql_drop_type; EXECUTE stmt_drop_type; DEALLOCATE PREPARE stmt_drop_type;