-- 000005 down：删除短信签名授权书字段（带存在性守卫，表/列缺失时跳过）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'auth_letter');
SET @sql = IF(@has_tbl > 0 AND @has_col > 0,
    'ALTER TABLE `oem_sms`.`sms_sign` DROP COLUMN `auth_letter`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
