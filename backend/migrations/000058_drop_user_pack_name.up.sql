-- 000058 用户已购资源包移除「资源包名称」列（快照冗余）
-- 背景：user_resource_pack.pack_name 仅为购买时的展示快照；展示口径改为按 product 标注产品类型，
--       表内保留价格与数量快照即可，无需再冗余名称。
-- 说明：禁止在迁移内 USE 切换会话库；跨库 DDL 一律反引号全限定库名.表名。幂等：表或列不存在时跳过。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_name');
SET @ddl = IF(@has_tbl > 0 AND @has_col > 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack` DROP COLUMN `pack_name`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_name');
SET @ddl = IF(@has_tbl > 0 AND @has_col > 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` DROP COLUMN `pack_name`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;