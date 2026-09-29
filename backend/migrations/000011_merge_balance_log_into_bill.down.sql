-- 000011 回滚（尽力而为）：
-- 重建 balance_log 表，把 bill 中 bank_serial_no 非空的行（人工充值，含历史迁移与合并后新增）迁回，
-- 再删除 bill.bank_serial_no 列与索引。
-- 注意：up 迁移中「非人工充值」的历史行（bank_serial_no 为空）在删除 balance_log 后无法精确还原，
-- 回滚后这部分仅存在于 bill，不重复写回 balance_log。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill');
SET @has_log_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'balance_log');
SET @has_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND COLUMN_NAME = 'bank_serial_no');

-- 重建 balance_log（与 000001 基线结构一致）
SET @sql_recreate = IF(@has_log_tbl = 0,
    'CREATE TABLE `oem_sys`.`balance_log` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `user_id` bigint NOT NULL,
        `order_id` bigint DEFAULT NULL,
        `type` tinyint NOT NULL,
        `amount` decimal(10,2) NOT NULL,
        `balance_before` decimal(10,2) NOT NULL,
        `balance_after` decimal(10,2) NOT NULL,
        `bank_serial_no` varchar(100) DEFAULT NULL,
        `remark` varchar(255) DEFAULT NULL,
        `created_at` datetime(3) DEFAULT NULL,
        PRIMARY KEY (`id`),
        KEY `idx_user_id` (`user_id`),
        KEY `idx_order_id` (`order_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''余额流水（已并入 bill，历史数据仅用于回滚）''',
    'SELECT 1');
PREPARE stmt_recreate FROM @sql_recreate; EXECUTE stmt_recreate; DEALLOCATE PREPARE stmt_recreate;

-- 迁回人工充值记录（bank_serial_no 非空）
SET @sql_migrate_back = IF(@has_tbl > 0 AND @has_log_tbl > 0 AND @has_serial > 0,
    'INSERT INTO `oem_sys`.`balance_log`
        (user_id, order_id, type, amount, balance_before, balance_after, bank_serial_no, remark, created_at)
     SELECT user_id, ref_id, bill_type, amount, balance_before, balance_after, bank_serial_no, remark, created_at
       FROM `oem_sys`.`bill`
      WHERE bank_serial_no IS NOT NULL AND bank_serial_no != ''''',
    'SELECT 1');
PREPARE stmt_migrate_back FROM @sql_migrate_back; EXECUTE stmt_migrate_back; DEALLOCATE PREPARE stmt_migrate_back;

-- 删除索引与列
SET @has_serial_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND INDEX_NAME = 'idx_bill_bank_serial_no');
SET @sql_drop_idx = IF(@has_tbl > 0 AND @has_serial_idx > 0,
    'ALTER TABLE `oem_sys`.`bill` DROP INDEX `idx_bill_bank_serial_no`',
    'SELECT 1');
PREPARE stmt_drop_idx FROM @sql_drop_idx; EXECUTE stmt_drop_idx; DEALLOCATE PREPARE stmt_drop_idx;

SET @sql_drop_serial = IF(@has_tbl > 0 AND @has_serial > 0,
    'ALTER TABLE `oem_sys`.`bill` DROP COLUMN `bank_serial_no`',
    'SELECT 1');
PREPARE stmt_drop_serial FROM @sql_drop_serial; EXECUTE stmt_drop_serial; DEALLOCATE PREPARE stmt_drop_serial;
