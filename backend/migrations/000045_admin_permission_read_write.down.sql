-- 000045 回滚：permissions 收回 varchar(512)
-- 注意：迁移时补齐的 .write 权限码不在此回滚（它们仍是合法权限码，语义正确；
--       若强行删除会把迁移后管理员手工授予的写权限一并抹掉）。回滚后如遇长度超出需人工处理。
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'permissions');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`admin_user` MODIFY COLUMN `permissions` varchar(512) NOT NULL DEFAULT '''' COMMENT ''权限码，逗号分隔，all=全部''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;