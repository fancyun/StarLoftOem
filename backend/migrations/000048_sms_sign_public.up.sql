-- 000048 短信签名新增公共标记：sms_sign.is_public（0-私有 1-公共）
-- 背景：验证码服务需要客户可使用平台提供的签名。改为在既有签名体系内实现：管理员在后台把已通过的签名标记为公共，
--       公共签名对所有账号可见、可被任意账号绑定模板使用；不再单独建平台签名表 / 后台页 / 下游接口。
-- 说明：
--   ① sms_sign 新增 is_public（默认 0=私有，存量行为不变）；
--   ② 清理早期方案遗留对象（sms_platform_sign 表、sms_template.platform_sign_id 列），若已被上一版 000048 建出则一并删除。
-- 禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。整份迁移幂等（表/列存在性判断）。

-- ① sms_sign 新增 is_public
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'is_public');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`sms_sign`
        ADD COLUMN `is_public` tinyint NOT NULL DEFAULT 0 COMMENT ''0-私有（仅提交账号可用）1-公共（所有账号可用）'' AFTER `status`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ② 清理早期方案遗留：sms_template.platform_sign_id
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_template' AND COLUMN_NAME = 'platform_sign_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`sms_template` DROP COLUMN `platform_sign_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ③ 清理早期方案遗留：sms_platform_sign 表
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_platform_sign');
SET @sql = IF(@has_tbl > 0,
    'DROP TABLE `oem_sms`.`sms_platform_sign`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;