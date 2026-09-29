-- 000010 回滚：删除短信上行回复表 sms_reply
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_reply');

SET @sql_drop = IF(@has_tbl > 0,
    'DROP TABLE `oem_sms`.`sms_reply`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;
