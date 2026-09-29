-- 000052 回滚：移除短信资源包的 product 类型列与索引。
-- 注意：已购营销短信资源包（product='sms_marketing'）的类型信息会随列删除丢失，回滚后两类包不再区分。
-- 幂等：按列/索引存在性判断。

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'resource_pack' AND INDEX_NAME = 'idx_product');
SET @sql_drop_idx_pack = IF(@has_idx > 0,
    'ALTER TABLE `oem_sms`.`resource_pack` DROP INDEX `idx_product`',
    'SELECT 1');
PREPARE stmt_drop_idx_pack FROM @sql_drop_idx_pack; EXECUTE stmt_drop_idx_pack; DEALLOCATE PREPARE stmt_drop_idx_pack;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND INDEX_NAME = 'idx_product');
SET @sql_drop_idx_user_pack = IF(@has_idx > 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` DROP INDEX `idx_product`',
    'SELECT 1');
PREPARE stmt_drop_idx_user_pack FROM @sql_drop_idx_user_pack; EXECUTE stmt_drop_idx_user_pack; DEALLOCATE PREPARE stmt_drop_idx_user_pack;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'resource_pack' AND COLUMN_NAME = 'product');
SET @sql_drop_pack = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`resource_pack` DROP COLUMN `product`',
    'SELECT 1');
PREPARE stmt_drop_pack FROM @sql_drop_pack; EXECUTE stmt_drop_pack; DEALLOCATE PREPARE stmt_drop_pack;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'product');
SET @sql_drop_user_pack = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` DROP COLUMN `product`',
    'SELECT 1');
PREPARE stmt_drop_user_pack FROM @sql_drop_user_pack; EXECUTE stmt_drop_user_pack; DEALLOCATE PREPARE stmt_drop_user_pack;