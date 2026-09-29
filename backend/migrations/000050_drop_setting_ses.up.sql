-- 000050 移除系统设置中已废弃的腾讯云 SES（邮箱）配置
-- 背景：注册与登录仅手机号/用户名，邮箱验证码与 SES 邮件已整体下线（见迁移 000040）；代码与 .env 中
--       相关键早已移除，仅配置表中残留历史种子行，本迁移一并清理。
-- 涉及键（历史分类 ses）：SES_FROM_EMAIL / SES_TEMPLATE_ID / SES_REGION。
-- 说明：纯数据清理，不改表结构。幂等：不存在则不影响。
--       禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');
SET @sql = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`setting`
        WHERE category = ''ses'' OR config_key IN (''SES_FROM_EMAIL'', ''SES_TEMPLATE_ID'', ''SES_REGION'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;