-- 000018 人工支付并入支付记录：payment_order 新增 bank_serial_no，历史人工充值按时间迁入支付记录，bill 去掉银行流水单号列
-- 背景：人工充值原先只在统一账单 bill 记一笔（bank_serial_no 承载唯一账单键），支付记录表看不到人工支付，
--       后台「支付记录」与首页收入统计缺失这部分资金。改为人工支付也落支付记录（渠道 manual、已支付），
--       银行流水单号随之从 bill 迁到 payment_order（人工支付仍照常写 bill 余额流水，两表通过 pay_order_id 关联）。
-- 迁移步骤：
--   1) payment_order 新增 bank_serial_no 列与索引
--   2) bill 中的人工充值记录（bank_serial_no 非空）按时间顺序插入 payment_order
--   3) 回填 bill.pay_order_id 指向对应人工支付单
--   4) 将 payment_order 全表按 created_at 重排 id 为 1..N，并同步更新 bill.pay_order_id 引用
--   5) 重置 AUTO_INCREMENT 为 max(id)+1（作为下一个支付记录 id 的起点）
--   6) 删除 bill.bank_serial_no 列（其索引随列一并删除）
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_bill_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill' AND COLUMN_NAME = 'bank_serial_no');

-- 1) payment_order 新增 bank_serial_no 列与索引
SET @has_po_serial = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'bank_serial_no');
SET @sql_add_col = IF(@has_po_serial = 0,
    'ALTER TABLE `oem_sys`.`payment_order`
        ADD COLUMN `bank_serial_no` varchar(100) NOT NULL DEFAULT '''' COMMENT ''银行流水单号（人工支付唯一账单键：账号_记账时间_交易流水号）'' AFTER `channel_trade_no`',
    'SELECT 1');
PREPARE stmt_add_col FROM @sql_add_col; EXECUTE stmt_add_col; DEALLOCATE PREPARE stmt_add_col;

SET @has_po_serial_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order' AND INDEX_NAME = 'idx_payment_order_bank_serial_no');
SET @sql_add_serial_idx = IF(@has_po_serial_idx = 0,
    'ALTER TABLE `oem_sys`.`payment_order` ADD INDEX `idx_payment_order_bank_serial_no` (`bank_serial_no`)',
    'SELECT 1');
PREPARE stmt_add_serial_idx FROM @sql_add_serial_idx; EXECUTE stmt_add_serial_idx; DEALLOCATE PREPARE stmt_add_serial_idx;

-- 2) 历史人工充值按时间迁入支付记录（人工支付：渠道 manual、已支付、支付时间取原入账时间）
--    支付流水号规则 M{入账时间yyyyMMddHHmmss}{账单自增号零填充6位}，与原 R 前缀的线上支付流水号区分且唯一
SET @sql_migrate = IF(@has_bill_serial > 0,
    'INSERT INTO `oem_sys`.`payment_order`
        (pay_order_no, user_id, amount, channel, channel_trade_no, bank_serial_no, status, paid_at,
         intent, biz_no, balance_amount, stock_reserved, pay_info, created_at, updated_at)
     SELECT CONCAT(''M'', DATE_FORMAT(b.created_at, ''%Y%m%d%H%i%s''), LPAD(b.id, 6, ''0'')),
            b.user_id, b.amount, ''manual'', '''', b.bank_serial_no, 1, b.created_at,
            ''recharge'', '''', 0, 0, '''', b.created_at, b.created_at
       FROM `oem_sys`.`bill` b
      WHERE b.bank_serial_no <> ''''
      ORDER BY b.created_at ASC, b.id ASC',
    'SELECT 1');
PREPARE stmt_migrate FROM @sql_migrate; EXECUTE stmt_migrate; DEALLOCATE PREPARE stmt_migrate;

-- 3) 账单回填关联的人工支付单（银行流水单号即人工支付唯一账单键）
SET @sql_link = IF(@has_bill_serial > 0,
    'UPDATE `oem_sys`.`bill` b
       JOIN `oem_sys`.`payment_order` po
         ON po.channel = ''manual'' AND po.bank_serial_no = b.bank_serial_no AND po.status = 1
        SET b.pay_order_id = po.id
      WHERE b.bank_serial_no <> ''''',
    'SELECT 1');
PREPARE stmt_link FROM @sql_link; EXECUTE stmt_link; DEALLOCATE PREPARE stmt_link;

-- 4) 支付记录按时间重排 id（1..N，新旧 id 映射存于临时表），并同步更新账单引用
DROP TEMPORARY TABLE IF EXISTS `tmp_po_id_map`;
CREATE TEMPORARY TABLE `tmp_po_id_map` AS
SELECT id AS old_id, ROW_NUMBER() OVER (ORDER BY created_at ASC, id ASC) AS new_id
  FROM `oem_sys`.`payment_order`;

UPDATE `oem_sys`.`bill` b
  JOIN `tmp_po_id_map` m ON m.old_id = b.pay_order_id
   SET b.pay_order_id = m.new_id
 WHERE b.pay_order_id > 0;

-- 先整体偏移到未被占用的 id 区间，再落到目标 id，避免重编号过程中的主键冲突
UPDATE `oem_sys`.`payment_order` po
  JOIN `tmp_po_id_map` m ON m.old_id = po.id
   SET po.id = m.new_id + 1000000;

UPDATE `oem_sys`.`payment_order` po
  JOIN `tmp_po_id_map` m ON m.new_id = po.id - 1000000
   SET po.id = m.new_id;

DROP TEMPORARY TABLE `tmp_po_id_map`;

-- 5) 重置自增起点为 max(id)+1
SET @po_max_id = (SELECT COALESCE(MAX(id), 0) FROM `oem_sys`.`payment_order`);
SET @sql_reset_ai = CONCAT('ALTER TABLE `oem_sys`.`payment_order` AUTO_INCREMENT = ', @po_max_id + 1);
PREPARE stmt_reset_ai FROM @sql_reset_ai; EXECUTE stmt_reset_ai; DEALLOCATE PREPARE stmt_reset_ai;

-- 6) 删除 bill.bank_serial_no（银行流水单号改由 payment_order 承载）
SET @sql_drop_serial = IF(@has_bill_serial > 0,
    'ALTER TABLE `oem_sys`.`bill` DROP COLUMN `bank_serial_no`',
    'SELECT 1');
PREPARE stmt_drop_serial FROM @sql_drop_serial; EXECUTE stmt_drop_serial; DEALLOCATE PREPARE stmt_drop_serial;