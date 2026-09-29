-- 000033 回滚：移除按产品分档的提成比例列（回落为单一 commission_rate）
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'commission_rate_sms');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP COLUMN `commission_rate_sms`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'commission_rate_fv');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`affiliate` DROP COLUMN `commission_rate_fv`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
