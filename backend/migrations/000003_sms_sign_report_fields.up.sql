-- 000003 短信签名报备扩展字段（联麓 shlianlu 报备需要）：
-- 签名来源 sign_type / 资质类型 label / 他公司主体与经办人信息 / 意愿承诺函与备案截图。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库——会破坏 golang-migrate 在系统库
-- oem_sys.schema_migrations 的版本记账（表现为 Dirty database version）。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_sign_type = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign' AND COLUMN_NAME = 'sign_type');
SET @sql = IF(@has_tbl > 0 AND @has_sign_type = 0,
    'ALTER TABLE `oem_sms`.`sms_sign`
        ADD COLUMN `sign_type` tinyint NOT NULL DEFAULT 1 COMMENT ''签名来源：1-本公司 2-他公司'' AFTER `id_card_back`,
        ADD COLUMN `label` tinyint NOT NULL DEFAULT 1 COMMENT ''资质类型：1-营业执照 2-商标'' AFTER `sign_type`,
        ADD COLUMN `company` varchar(128) NOT NULL DEFAULT '''' COMMENT ''公司名称（他公司必填）'' AFTER `label`,
        ADD COLUMN `legal_person` varchar(64) NOT NULL DEFAULT '''' COMMENT ''法人姓名（他公司必填）'' AFTER `company`,
        ADD COLUMN `credit_code` varchar(64) NOT NULL DEFAULT '''' COMMENT ''统一社会信用代码（他公司必填）'' AFTER `legal_person`,
        ADD COLUMN `credit_user_name` varchar(64) NOT NULL DEFAULT '''' COMMENT ''经办人姓名（他公司必填）'' AFTER `credit_code`,
        ADD COLUMN `id_card` varchar(64) NOT NULL DEFAULT '''' COMMENT ''经办人身份证号（他公司必填）'' AFTER `credit_user_name`,
        ADD COLUMN `phone` varchar(32) NOT NULL DEFAULT '''' COMMENT ''经办人手机号（他公司必填）'' AFTER `id_card`,
        ADD COLUMN `sx_commits` varchar(512) NOT NULL DEFAULT '''' COMMENT ''用户接收意愿承诺函 URL'' AFTER `phone`,
        ADD COLUMN `screenshot` varchar(512) NOT NULL DEFAULT '''' COMMENT ''商标备案截图 URL'' AFTER `sx_commits`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
