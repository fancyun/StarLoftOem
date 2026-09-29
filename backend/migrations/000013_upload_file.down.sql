-- 000013 回滚：删除用户上传文件登记表 upload_file
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'upload_file');

SET @sql_drop = IF(@has_tbl > 0,
    'DROP TABLE `oem_sys`.`upload_file`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;