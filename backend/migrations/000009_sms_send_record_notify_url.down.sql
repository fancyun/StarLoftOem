-- 000009 回滚：删除短信发送记录的 notify_url 列
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record');

SET @has_notify_url = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record' AND COLUMN_NAME = 'notify_url');
SET @sql_notify_url = IF(@has_tbl > 0 AND @has_notify_url > 0,
    'ALTER TABLE `oem_sms`.`sms_send_record` DROP COLUMN `notify_url`',
    'SELECT 1');
PREPARE stmt_notify_url FROM @sql_notify_url; EXECUTE stmt_notify_url; DEALLOCATE PREPARE stmt_notify_url;
