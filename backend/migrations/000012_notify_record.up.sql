-- 000012 新建下游通知重试记录表 notify_record
-- 通知下游（FV结果/短信回执/短信回复）失败时落库，定时任务按指数退避补推，成功置 1、超限置 2 放弃。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补建。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record');

SET @sql_create = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`notify_record` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `biz_type` varchar(32) NOT NULL COMMENT ''业务类型：fv_result/sms_receipt/sms_reply'',
        `record_id` bigint NOT NULL COMMENT ''关联业务记录ID'',
        `user_id` bigint NOT NULL DEFAULT 0 COMMENT ''归属用户'',
        `target_url` varchar(500) NOT NULL DEFAULT '''' COMMENT ''下游通知地址'',
        `payload` text COMMENT ''推送报文JSON（含签名）'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待推 1-成功 2-已放弃'',
        `fail_times` int NOT NULL DEFAULT 0 COMMENT ''连续失败次数'',
        `last_error` varchar(255) NOT NULL DEFAULT '''' COMMENT ''最近失败原因'',
        `next_retry_at` datetime NULL COMMENT ''下次重推时间'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_status_next` (`status`, `next_retry_at`),
        KEY `idx_biz` (`biz_type`, `record_id`),
        KEY `idx_user_id` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''下游通知重试记录''',
    'SELECT 1');
PREPARE stmt_create FROM @sql_create; EXECUTE stmt_create; DEALLOCATE PREPARE stmt_create;
