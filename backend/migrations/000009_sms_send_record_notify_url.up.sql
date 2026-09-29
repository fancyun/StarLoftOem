-- 000009 短信发送记录新增下游回执主动推送地址 notify_url
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record');

SET @has_notify_url = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_send_record' AND COLUMN_NAME = 'notify_url');
SET @sql_notify_url = IF(@has_tbl > 0 AND @has_notify_url = 0,
    'ALTER TABLE `oem_sms`.`sms_send_record`
        ADD COLUMN `notify_url` varchar(500) NOT NULL DEFAULT '''' COMMENT ''下游回执主动推送地址（发送时传入）'' AFTER `request_id`',
    'SELECT 1');
PREPARE stmt_notify_url FROM @sql_notify_url; EXECUTE stmt_notify_url; DEALLOCATE PREPARE stmt_notify_url;
