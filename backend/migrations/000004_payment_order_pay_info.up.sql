-- 000004 支付单新增渠道支付信息列：保存支付宝 pay_url / 微信 code_url / h5_url 等 JSON。
-- 同一用户对同一笔支付重复发起时直接复用未过期的待支付单（微信/支付宝 out_trade_no 不可重复下单，
-- 渠道侧会返回 OUT_TRADE_NO_USED），并返回首次下单时保存的支付信息。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'pay_info');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`payment_order` ADD COLUMN `pay_info` TEXT NULL COMMENT ''渠道支付信息 JSON（支付宝 pay_url/微信 code_url/h5_url）'' AFTER `stock_reserved`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
