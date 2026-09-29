-- 000049 移除系统设置中的 KYC_PRICE（原「有源/无源人脸核验单价未配置时的兜底单价」）
-- 背景：该键不再由后台维护——人脸核验单价由「人脸核验 → 产品配置」的 fv_auth_price / fv_self_price 提供，
--       账户实名单价只保留 KYC_PERSONAL_PRICE / KYC_ENTERPRISE_PRICE。故从配置目录（config.SettingCatalog）与配置表中一并移除。
-- 说明：纯数据清理，不改表结构。幂等：仅删除存在的键。
--       禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');
SET @sql = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`setting` WHERE config_key = ''KYC_PRICE''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;