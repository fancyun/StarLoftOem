-- 000041 管理员/员工权限：admin_user 新增 permissions 列（逗号分隔权限码，all=全部）
-- 背景：平台只有一个超级管理员，无法多人协作；新增员工账号后需按后台功能逐项授权。
-- 处理：① 新增 permissions 列；② 存量管理员回填 'all'，避免升级后因权限为空被全部拒绝（含默认超管）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（列存在性判断）。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'permissions');
SET @sql_add = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`admin_user`
        ADD COLUMN `permissions` varchar(512) NOT NULL DEFAULT '''' COMMENT ''权限码，逗号分隔，all=全部'' AFTER `status`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;

-- 存量账号（含默认超管）授予全部权限
UPDATE `oem_sys`.`admin_user` SET `permissions` = 'all' WHERE `permissions` = '';