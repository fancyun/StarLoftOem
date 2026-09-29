-- 000023 实名命名统一：user 表列重命名（is_kyc_verified → realname_status，kyc_free_base → personal_free_base）
-- 背景：is_kyc_verified 同时表达个人实名(1)与企业实名(2)，字段名只体现 kyc；个人实名免费次数基准 kyc_free_base
--       与企业侧 enterprise_free_base 前缀不对称。统一为 realname_status（实名状态，语义中立）与 personal_free_base。
-- 存量库按信息架构检测补列，全新库表结构由 AutoMigrate 直接建新列名（此时旧列不存在，跳过）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');

SET @has_old_status = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'is_kyc_verified');
SET @has_new_status = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'realname_status');
SET @sql_status = IF(@has_tbl > 0 AND @has_old_status > 0 AND @has_new_status = 0,
    'ALTER TABLE `oem_sys`.`user` CHANGE COLUMN `is_kyc_verified` `realname_status` tinyint NOT NULL DEFAULT 0 COMMENT ''实名状态：0-未实名 1-个人实名(kyc) 2-企业实名(kyb)''',
    'SELECT 1');
PREPARE stmt_status FROM @sql_status; EXECUTE stmt_status; DEALLOCATE PREPARE stmt_status;

SET @has_old_base = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'kyc_free_base');
SET @has_new_base = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'personal_free_base');
SET @sql_base = IF(@has_tbl > 0 AND @has_old_base > 0 AND @has_new_base = 0,
    'ALTER TABLE `oem_sys`.`user` CHANGE COLUMN `kyc_free_base` `personal_free_base` int NOT NULL DEFAULT 0 COMMENT ''个人实名免费次数基准偏移''',
    'SELECT 1');
PREPARE stmt_base FROM @sql_base; EXECUTE stmt_base; DEALLOCATE PREPARE stmt_base;