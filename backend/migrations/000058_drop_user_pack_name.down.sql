-- 000058 回滚：恢复 user_resource_pack 的「资源包名称」列
-- 说明：仅恢复列结构，名称内容无法还原，存量行置空串。幂等：表或列不存在时跳过。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_name');
SET @ddl = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack` ADD COLUMN `pack_name` varchar(100) NOT NULL DEFAULT '''' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_name');
SET @ddl = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` ADD COLUMN `pack_name` varchar(100) NOT NULL DEFAULT '''' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;