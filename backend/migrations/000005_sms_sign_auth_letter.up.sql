-- 000005 短信签名报备新增「短信签名授权书」字段（他公司签名报备材料）。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库——会破坏 golang-migrate 在系统库
-- oem_sys.schema_migrations 的版本记账（表现为 Dirty database version）。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_auth_letter = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'auth_letter');
SET @sql = IF(@has_tbl > 0 AND @has_auth_letter = 0,
    'ALTER TABLE `oem_sms`.`sms_sign`
        ADD COLUMN `auth_letter` varchar(512) NOT NULL DEFAULT '''' COMMENT ''短信签名授权书 URL'' AFTER `sx_commits`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
