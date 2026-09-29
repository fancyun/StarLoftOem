-- 000018 回滚：bill 恢复 bank_serial_no 列，人工支付记录退出支付记录表
-- 说明：payment_order.id 在升级中已按时间重排，回滚不回退编号（id 仅作标识，业务以 pay_order_no 为准）。
-- 步骤：
--   1) bill 重新加回 bank_serial_no 列与索引
--   2) 人工支付单的银行流水单号回填到对应账单，并解除账单与该支付单的绑定
--   3) 删除人工支付记录（channel=manual）
--   4) 删除 payment_order.bank_serial_no 列（索引随列一并删除）
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_po_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'bank_serial_no');

-- 1) bill 恢复 bank_serial_no 列与索引
SET @has_bill_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND COLUMN_NAME = 'bank_serial_no');
SET @sql_add_serial = IF(@has_bill_serial = 0,
    'ALTER TABLE `oem_sys`.`bill`
        ADD COLUMN `bank_serial_no` varchar(100) NOT NULL DEFAULT '''' COMMENT ''银行流水单号（人工充值）'' AFTER `pay_order_id`',
    'SELECT 1');
PREPARE stmt_add_serial FROM @sql_add_serial; EXECUTE stmt_add_serial; DEALLOCATE PREPARE stmt_add_serial;

SET @has_bill_serial_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND INDEX_NAME = 'idx_bill_bank_serial_no');
SET @sql_add_serial_idx = IF(@has_bill_serial_idx = 0,
    'ALTER TABLE `oem_sys`.`bill` ADD INDEX `idx_bill_bank_serial_no` (`bank_serial_no`)',
    'SELECT 1');
PREPARE stmt_add_serial_idx FROM @sql_add_serial_idx; EXECUTE stmt_add_serial_idx; DEALLOCATE PREPARE stmt_add_serial_idx;

-- 2) 银行流水单号回填账单并解除与人工支付单的绑定
SET @sql_unlink = IF(@has_po_serial > 0,
    'UPDATE `oem_sys`.`bill` b
       JOIN `oem_sys`.`payment_order` po ON po.id = b.pay_order_id AND po.channel = ''manual''
        SET b.bank_serial_no = po.bank_serial_no, b.pay_order_id = 0',
    'SELECT 1');
PREPARE stmt_unlink FROM @sql_unlink; EXECUTE stmt_unlink; DEALLOCATE PREPARE stmt_unlink;

-- 3) 删除人工支付记录
DELETE FROM `oem_sys`.`payment_order` WHERE channel = 'manual';

-- 4) 删除 payment_order.bank_serial_no（银行流水单号回到 bill 承载）
SET @sql_drop_serial = IF(@has_po_serial > 0,
    'ALTER TABLE `oem_sys`.`payment_order` DROP COLUMN `bank_serial_no`',
    'SELECT 1');
PREPARE stmt_drop_serial FROM @sql_drop_serial; EXECUTE stmt_drop_serial; DEALLOCATE PREPARE stmt_drop_serial;