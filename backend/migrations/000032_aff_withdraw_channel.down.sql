-- 000032 回滚：移除提现方式列
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'channel');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw` DROP COLUMN `channel`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
