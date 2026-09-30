-- 000057 回滚：恢复 notify_record 的自增 id 主键（结构回滚，成功即删的历史行无法恢复）
-- 说明：新增 id 主键并保留 biz_no 列；按 (biz_type, biz_no) 的唯一性回归由应用层保证（回滚后旧代码不再按 biz_type+biz_no 去重）。
-- 幂等：各步先判断是否已执行；表不存在时跳过。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record');

SET @has_pk = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record'
      AND INDEX_NAME = 'PRIMARY' AND COLUMN_NAME = 'biz_no');
SET @ddl = IF(@has_tbl > 0 AND @has_pk > 0,
    'ALTER TABLE `oem_sys`.`notify_record` DROP PRIMARY KEY',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND COLUMN_NAME = 'id');
SET @ddl = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`notify_record` ADD COLUMN `id` bigint NOT NULL AUTO_INCREMENT PRIMARY KEY FIRST',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;