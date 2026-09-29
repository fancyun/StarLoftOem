-- 000016 回滚：删除 sms_send_record.pack_id
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record' AND COLUMN_NAME = 'pack_id');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`sms_send_record` DROP COLUMN `pack_id`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;