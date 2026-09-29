-- 000040 移除 user.email 列（平台不再支持邮箱：注册与登录仅手机号/用户名）
-- 背景：邮箱验证码与邮箱登录整体下线，user.email 及其唯一索引不再使用，平台也不再收集邮箱，
--       故直接删列（唯一索引随列一并删除），历史邮箱数据不再保留。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（列不存在时跳过）。
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'email');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `email`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;