-- 000021 删除 kyc.source 列
-- 背景：实名记录表已只存账户实名（下游 API 调用记录走 oem_fv.auth_order，见迁移 000020），
--       来源列取值恒为 1，已无意义，故删除。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND COLUMN_NAME = 'source');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`kyc` DROP COLUMN `source`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;