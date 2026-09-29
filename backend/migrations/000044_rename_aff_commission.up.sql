-- 000044 提成流水表命名对齐：aff_commission 更名为 user_commission
-- 背景：员工销售提成表为 staff_commission，用户型推广提成表原名 aff_commission，
--       两张表命名不对齐；更名为 user_commission，使「用户型推广 / 员工销售」两两对应。
-- 处理：① 旧表存在且新表名未被占用时 RENAME TABLE；
--       ② 索引名同步对齐为 idx_user_settle_oem / idx_user_settle_user（与 staff 侧命名一致）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（存在性判断）。

-- ① 表名对齐
SET @has_old = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_commission');
SET @has_new = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission');
SET @sql = IF(@has_old > 0 AND @has_new = 0,
    'RENAME TABLE `oem_sys`.`aff_commission` TO `oem_sys`.`user_commission`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ② 索引名对齐（MySQL 5.7+ 支持 RENAME INDEX）
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_oem_settle_oem');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` RENAME INDEX `idx_oem_settle_oem` TO `idx_user_settle_oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_commission' AND INDEX_NAME = 'idx_oem_settle_user');
SET @sql = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_commission` RENAME INDEX `idx_oem_settle_user` TO `idx_user_settle_user`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;