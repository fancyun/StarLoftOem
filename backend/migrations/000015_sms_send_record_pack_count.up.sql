-- 000015 短信发送记录新增 pack_count（本次扣减的资源包条数，余额支付为 0）
-- 背景：资源包扣量时 amount 不再记录折算金额（改为 0），另用 pack_count 体现扣了多少条资源包，便于用户对账。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record' AND COLUMN_NAME = 'pack_count');
SET @sql_add = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`sms_send_record`
        ADD COLUMN `pack_count` int NOT NULL DEFAULT 0 COMMENT ''本次扣减的资源包条数（余额支付为 0）'' AFTER `pay_type`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;

-- 存量回填：历史上资源包支付的记录按计费条数回填扣减条数（金额列保留原值，不回改历史）
SET @sql_fix = IF(@has_tbl > 0 AND @has_col = 0,
    'UPDATE `oem_sms`.`sms_send_record` SET pack_count = phone_count WHERE pay_type = 0 AND status = 0',
    'SELECT 1');
PREPARE stmt_fix FROM @sql_fix; EXECUTE stmt_fix; DEALLOCATE PREPARE stmt_fix;