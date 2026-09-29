-- 000016 短信发送记录新增 pack_id（本次扣减的资源包 ID）
-- 背景：回执失败需要把已扣的资源包条数退回原包，而记录里只存了条数（pack_count），
--       故补记资源包 ID；历史行无法追溯，保持 0（退款时回落用户最近一条资源包）。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record' AND COLUMN_NAME = 'pack_id');
SET @sql_add = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`sms_send_record`
        ADD COLUMN `pack_id` bigint NOT NULL DEFAULT 0 COMMENT ''本次扣减的资源包 ID（余额支付为 0）'' AFTER `pack_count`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;