-- 000046 配置表只保留非密钥类：清理历史遗留的密钥类配置行
-- 背景：密钥类配置改为只由 .env 提供（判定见 config.IsSecretSettingKey），配置表不再承载密钥，
--       故删除历史已入库的密钥行（如 TENCENT_SECRET_KEY / TENCENT_CAPTCHA_SECRET / SHLIANLU_KEY /
--       FINAUTH_API_KEY / FINAUTH_API_SECRET / WECHAT_API_V3_KEY 等）。
--       删除后这些键回落 .env 取值，行为不变；密钥类也不再在后台「系统设置」展示与维护。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（重复执行无副作用）。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');

-- 判定与 config.IsSecretSettingKey 一致：键名含 SECRET / PASSWORD / KEY
-- （PRIVATE_KEY / APIKEY / API_KEY 均含 KEY，已被覆盖）
SET @sql = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`setting` WHERE UPPER(config_key) LIKE ''%SECRET%'' OR UPPER(config_key) LIKE ''%PASSWORD%'' OR UPPER(config_key) LIKE ''%KEY%''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;