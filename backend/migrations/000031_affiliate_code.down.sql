-- 000031 回滚：移除推广商推广码列与唯一索引
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND INDEX_NAME = 'uk_affiliate_code');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP INDEX `uk_affiliate_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP COLUMN `aff_code`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
