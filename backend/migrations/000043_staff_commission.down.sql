-- 000043 回滚：把员工提成行移回 aff_commission，并删除 staff_commission
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @sql = IF(@has_tbl > 0,
    'INSERT INTO `oem_sys`.`aff_commission` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
     SELECT id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at FROM `oem_sys`.`staff_commission`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS `oem_sys`.`staff_commission`;