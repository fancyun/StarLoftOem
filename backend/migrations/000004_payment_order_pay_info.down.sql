-- 000004 down：删除支付信息列（带存在性守卫，表/列缺失时跳过）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'pay_info');
SET @sql = IF(@has_tbl > 0 AND @has_col > 0,
    'ALTER TABLE `oem_sys`.`payment_order` DROP COLUMN `pay_info`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
