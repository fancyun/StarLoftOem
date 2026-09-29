-- 000027 回滚：移除 OEM 分销与配置体系相关表与字段
-- 说明：回滚会丢失 OEM 归属、定价与结算数据，执行前请确认已备份。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS `oem_sys`.`oem_settlement`;
DROP TABLE IF EXISTS `oem_sys`.`oem`;
DROP TABLE IF EXISTS `oem_sys`.`price_override`;
DROP TABLE IF EXISTS `oem_sys`.`setting`;
DROP TABLE IF EXISTS `oem_fv`.`product_config`;
DROP TABLE IF EXISTS `oem_sms`.`product_config`;