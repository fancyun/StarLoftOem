-- 000040 回滚：恢复 user.email 列。
-- 注意：历史邮箱数据已删除、无法还原，列值只能为空串；因此这里只建普通索引（非唯一）——
--       多行空串无法满足唯一约束，原唯一索引 `idx_user_email` 不再重建。
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'email');

SET @sql_add = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`user` ADD COLUMN `email` varchar(100) NOT NULL DEFAULT '''' AFTER `username`, ADD INDEX `idx_user_email` (`email`)',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;