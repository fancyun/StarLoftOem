-- 000022 删除 auth_order.source 列
-- 背景：账户实名已不再产生认证订单（走 oem_sys.kyc + 腾讯云人脸核身），
--       认证订单只由下游 API 人脸核验产生，来源列取值恒为 2，已无意义，故删除。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'source');

SET @sql_drop = IF(@has_col > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `source`',
    'SELECT 1');
PREPARE stmt_drop FROM @sql_drop; EXECUTE stmt_drop; DEALLOCATE PREPARE stmt_drop;