-- 000048 回滚：删除 sms_sign.is_public
-- 说明：早期方案遗留的平台签名表 / 模板列不在此恢复（属已被取代的中间产物）。
--       禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'is_public');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`sms_sign` DROP COLUMN `is_public`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;