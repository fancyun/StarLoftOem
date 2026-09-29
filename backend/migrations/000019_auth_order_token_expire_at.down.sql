-- 000018 回滚：删除 auth_order.token_expire_at
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'token_expire_at');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `token_expire_at`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;