-- 000041 回滚：删除 admin_user.permissions 列（权限信息随列一并丢弃，无法还原）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'permissions');
SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`admin_user` DROP COLUMN `permissions`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;