-- 000002 重命名实名主体字段：kyc_name/kyc_number -> verified_name/verified_number
-- 字段同时承载个人实名（姓名/身份证号）与企业实名（企业名称/统一社会信用代码），
-- 用 verified_* 更贴切。旧库存在原列则重命名；全新库（AutoMigrate 之后建表）无原列则跳过。
-- 注意：禁止在迁移文件内使用 USE 切换会话库；跨库 DDL 一律用反引号全限定库名.表名。

SET @has_kyc_name = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'kyc_name');
SET @sql = IF(@has_kyc_name > 0,
    'ALTER TABLE `oem_sys`.`user` RENAME COLUMN `kyc_name` TO `verified_name`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_kyc_number = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'kyc_number');
SET @sql = IF(@has_kyc_number > 0,
    'ALTER TABLE `oem_sys`.`user` RENAME COLUMN `kyc_number` TO `verified_number`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
