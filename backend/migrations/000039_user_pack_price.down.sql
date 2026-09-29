-- 000039 回滚：恢复 pack_id 列、移除 price 列。
-- 注意：pack_id 的历史值已无法恢复（回滚后统一为 0）；price 快照数据会随列删除丢失。
-- 幂等：按列存在性判断。

-- 恢复 pack_id（人脸核验库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_id');
SET @sql_add_fv = IF(@has_col = 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack`
        ADD COLUMN `pack_id` bigint NOT NULL DEFAULT 0 COMMENT ''资源包 ID（已废弃，改记价格快照）'' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt_add_fv FROM @sql_add_fv; EXECUTE stmt_add_fv; DEALLOCATE PREPARE stmt_add_fv;

-- 恢复 pack_id（短信库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_id');
SET @sql_add_sms = IF(@has_col = 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack`
        ADD COLUMN `pack_id` bigint NOT NULL DEFAULT 0 COMMENT ''资源包 ID（已废弃，改记价格快照）'' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt_add_sms FROM @sql_add_sms; EXECUTE stmt_add_sms; DEALLOCATE PREPARE stmt_add_sms;

-- 删除 price（人脸核验库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'price');
SET @sql_drop_fv = IF(@has_col > 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack` DROP COLUMN `price`',
    'SELECT 1');
PREPARE stmt_drop_fv FROM @sql_drop_fv; EXECUTE stmt_drop_fv; DEALLOCATE PREPARE stmt_drop_fv;

-- 删除 price（短信库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'price');
SET @sql_drop_sms = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` DROP COLUMN `price`',
    'SELECT 1');
PREPARE stmt_drop_sms FROM @sql_drop_sms; EXECUTE stmt_drop_sms; DEALLOCATE PREPARE stmt_drop_sms;