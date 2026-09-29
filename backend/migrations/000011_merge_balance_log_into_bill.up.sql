-- 000011 余额流水并入账单：bill 加 bank_serial_no，迁移历史 balance_log 后删表
-- 1) bill 新增 bank_serial_no（人工充值银行流水号，普通索引防重复扫描；不设唯一索引避免历史脏数据导致迁移失败）
-- 2) 历史 balance_log 扁平化迁入 bill（recharge/balance/refund 三类；仅迁移在 bill 中无对应双写记录的行，
--    双写去重匹配：user_id + amount + balance_before/after 一致且 created_at 相差 60 秒内）
-- 3) 删除 balance_log 表
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill');

SET @has_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND COLUMN_NAME = 'bank_serial_no');
SET @sql_add_serial = IF(@has_tbl > 0 AND @has_serial = 0,
    'ALTER TABLE `oem_sys`.`bill`
        ADD COLUMN `bank_serial_no` varchar(100) NOT NULL DEFAULT '''' COMMENT ''银行流水单号（人工充值）'' AFTER `pay_order_id`',
    'SELECT 1');
PREPARE stmt_add_serial FROM @sql_add_serial; EXECUTE stmt_add_serial; DEALLOCATE PREPARE stmt_add_serial;

SET @has_serial_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND INDEX_NAME = 'idx_bill_bank_serial_no');
SET @sql_add_serial_idx = IF(@has_tbl > 0 AND @has_serial_idx = 0,
    'ALTER TABLE `oem_sys`.`bill` ADD INDEX `idx_bill_bank_serial_no` (`bank_serial_no`)',
    'SELECT 1');
PREPARE stmt_add_serial_idx FROM @sql_add_serial_idx; EXECUTE stmt_add_serial_idx; DEALLOCATE PREPARE stmt_add_serial_idx;

-- 迁移历史 balance_log（跳过与 bill 双写的记录）
SET @has_log_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'balance_log');
SET @sql_migrate = IF(@has_tbl > 0 AND @has_log_tbl > 0,
    'INSERT INTO `oem_sys`.`bill`
        (biz_no, user_id, product, service, bill_type, spend_type, pay_type, amount,
         balance_before, balance_after, ref_type, ref_id, ref_biz_no, pay_order_id, remark, bank_serial_no, created_at)
     SELECT LPAD(b.id, 20, ''0''), b.user_id, '''',
            CASE b.type WHEN 1 THEN ''recharge'' WHEN 2 THEN ''balance'' ELSE ''refund'' END,
            b.type,
            CASE b.type WHEN 1 THEN ''recharge'' WHEN 2 THEN ''balance'' ELSE ''refund'' END,
            1,
            b.amount, b.balance_before, b.balance_after,
            '''', b.order_id, '''', 0, b.remark, b.bank_serial_no, b.created_at
       FROM `oem_sys`.`balance_log` b
      WHERE NOT EXISTS (
            SELECT 1 FROM `oem_sys`.`bill` x
             WHERE x.user_id = b.user_id
               AND x.amount = b.amount
               AND x.balance_before = b.balance_before
               AND x.balance_after = b.balance_after
               AND ABS(TIMESTAMPDIFF(SECOND, x.created_at, b.created_at)) <= 60
      )',
    'SELECT 1');
PREPARE stmt_migrate FROM @sql_migrate; EXECUTE stmt_migrate; DEALLOCATE PREPARE stmt_migrate;

-- 删除 balance_log 表
SET @sql_drop_log = IF(@has_log_tbl > 0,
    'DROP TABLE `oem_sys`.`balance_log`',
    'SELECT 1');
PREPARE stmt_drop_log FROM @sql_drop_log; EXECUTE stmt_drop_log; DEALLOCATE PREPARE stmt_drop_log;
