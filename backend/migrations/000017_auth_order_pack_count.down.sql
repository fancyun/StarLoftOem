-- 000016 回滚：删除 auth_order.pack_count
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'pack_count');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `pack_count`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;