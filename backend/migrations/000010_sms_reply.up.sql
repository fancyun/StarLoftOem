-- 000010 短信库新增上行回复表 sms_reply（回复推送落库 / 按日拉取回复落库）
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补建。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_reply');

SET @sql_create = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sms`.`sms_reply` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `user_id` bigint NOT NULL COMMENT ''归属用户（按 taskId 关联发送记录）'',
        `task_id` varchar(64) NOT NULL DEFAULT '''' COMMENT ''上游任务 ID（关联发送记录 message_sid）'',
        `phone` varchar(32) NOT NULL DEFAULT '''' COMMENT ''接收号码（回复来源）'',
        `sequence_id` varchar(64) NOT NULL DEFAULT '''' COMMENT ''序列 ID（推送有，唯一）'',
        `content_down` text COMMENT ''下行短信内容'',
        `content_up` text COMMENT ''回复内容'',
        `resp_time` varchar(32) NOT NULL DEFAULT '''' COMMENT ''回复时间（拉取为日期时间，推送为时间戳）'',
        `status` varchar(20) NOT NULL DEFAULT '''' COMMENT ''推送状态'',
        `tag` varchar(64) NOT NULL DEFAULT '''' COMMENT ''自定义标签（原样回传）'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_sequence_id` (`sequence_id`),
        KEY `idx_user_id` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''短信上行回复记录''',
    'SELECT 1');
PREPARE stmt_create FROM @sql_create; EXECUTE stmt_create; DEALLOCATE PREPARE stmt_create;
