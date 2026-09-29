-- 000003 回滚：移除短信签名报备扩展字段
-- 注意：禁止在迁移文件内使用 USE 切换会话库（破坏 golang-migrate 版本记账），跨库 DDL 用全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_sign_type = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'sign_type');
SET @sql = IF(@has_tbl > 0 AND @has_sign_type > 0,
    'ALTER TABLE `oem_sms`.`sms_sign`
        DROP COLUMN `screenshot`,
        DROP COLUMN `sx_commits`,
        DROP COLUMN `phone`,
        DROP COLUMN `id_card`,
        DROP COLUMN `credit_user_name`,
        DROP COLUMN `credit_code`,
        DROP COLUMN `legal_person`,
        DROP COLUMN `company`,
        DROP COLUMN `label`,
        DROP COLUMN `sign_type`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
