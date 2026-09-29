-- 000006 回滚：删除 auth_order.best_img_fetched 列
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'best_img_fetched');
SET @sql = IF(@has_tbl > 0 AND @has_col > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `best_img_fetched`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
