-- 000014 修复 sms_template 历史 NULL 值，避免查询模板时报 converting NULL to string is unsupported
-- 背景：sign_id/sign_name/channel/template_id/reason 为可空列，历史行存在 NULL，
--       Go 侧 Scan 到 NULL 无法写入 string/int64，导致「查询结果」等接口报错。
--       统一回填为 0/空串，与现有代码写入（一律写空值）保持一致。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_template');

SET @sql_fix = IF(@has_tbl > 0,
    'UPDATE `oem_sms`.`sms_template` SET
        sign_id = COALESCE(sign_id, 0),
        sign_name = COALESCE(sign_name, ''''),
        channel = COALESCE(channel, ''''),
        template_id = COALESCE(template_id, ''''),
        reason = COALESCE(reason, '''')
     WHERE sign_id IS NULL OR sign_name IS NULL OR channel IS NULL OR template_id IS NULL OR reason IS NULL',
    'SELECT 1');
PREPARE stmt_fix FROM @sql_fix; EXECUTE stmt_fix; DEALLOCATE PREPARE stmt_fix;