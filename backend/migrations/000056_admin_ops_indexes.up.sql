-- 000056 后台运维页所需索引（登录日志时间、推广归属、通知重试调度）
-- 背景：新增后台「日志与审计 / 通知重试 / 推广归属」页面后：
--       - user_login_log / admin_login_log 只有 user_id/account 索引，日期区间筛选会全表扫描；
--       - user.referrer_type/referrer_id 无索引，下级列表与推广归属聚合为全表扫描；
--       - notify_record 的重试调度查询（status + next_retry_at）无索引。
-- 说明：禁止在迁移内 USE 切换会话库；跨库 DDL 一律反引号全限定库名.表名。
--       幂等：表不存在（全新库，表结构由 AutoMigrate 在迁移之后补齐）或索引已存在时跳过。
--       模型侧已在对应字段声明同名 index tag（AutoMigrate 见到同名索引会跳过，不会重复创建）。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_login_log');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user_login_log' AND INDEX_NAME = 'idx_created_at');
SET @ddl = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user_login_log` ADD INDEX `idx_created_at` (`created_at`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_login_log');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_login_log' AND INDEX_NAME = 'idx_created_at');
SET @ddl = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`admin_login_log` ADD INDEX `idx_created_at` (`created_at`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'idx_referrer');
SET @ddl = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`user` ADD INDEX `idx_referrer` (`referrer_type`, `referrer_id`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record');
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND INDEX_NAME = 'idx_status_next');
SET @ddl = IF(@has_tbl > 0 AND @has_idx = 0,
    'ALTER TABLE `oem_sys`.`notify_record` ADD INDEX `idx_status_next` (`status`, `next_retry_at`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;