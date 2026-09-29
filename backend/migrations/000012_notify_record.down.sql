-- 000012 回滚：删除下游通知重试记录表 notify_record
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record');

SET @sql_drop = IF(@has_tbl > 0,
    'DROP TABLE `oem_sys`.`notify_record`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;
