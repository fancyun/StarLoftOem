-- 000056 回滚：移除后台运维页新增索引
-- 说明：仅回滚索引，数据不受影响。幂等：索引不存在时跳过。

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_login_log' AND INDEX_NAME = 'idx_created_at');
SET @ddl = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user_login_log` DROP INDEX `idx_created_at`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_login_log' AND INDEX_NAME = 'idx_created_at');
SET @ddl = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`admin_login_log` DROP INDEX `idx_created_at`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_referrer');
SET @ddl = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user` DROP INDEX `idx_referrer`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND INDEX_NAME = 'idx_status_next');
SET @ddl = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`notify_record` DROP INDEX `idx_status_next`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;