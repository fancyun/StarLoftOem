-- 000044 回滚：user_commission 更名回 aff_commission，索引名还原
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（存在性判断）。

-- 索引名还原（先还原索引，再还原表名，保证回滚后结构与 000043 时点一致）
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` RENAME INDEX `idx_user_settle_oem` TO `idx_oem_settle_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_user_settle_user');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` RENAME INDEX `idx_user_settle_user` TO `idx_oem_settle_user`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 表名还原
SET @has_new = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @has_old = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_commission');
SET @sql = IF(@has_new > 0 AND @has_old = 0,
    'RENAME TABLE `oem_sys`.`user_commission` TO `oem_sys`.`aff_commission`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;