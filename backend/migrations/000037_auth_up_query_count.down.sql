-- 000037 回滚：删除 auth_record.up_query_count
-- 说明：禁止在迁移内 USE 切换会话库；跨库 DDL 一律全限定库名.表名。幂等。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_record');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_record' AND COLUMN_NAME = 'up_query_count');
SET @sql_drop = IF(@has_tbl > 0 AND @has_col = 1,
    'ALTER TABLE `oem_fv`.`auth_record` DROP COLUMN `up_query_count`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;
