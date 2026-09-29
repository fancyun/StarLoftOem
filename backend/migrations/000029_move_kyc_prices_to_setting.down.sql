-- 000029 回滚：把账户实名单价从系统配置搬回人脸核验产品配置
-- 说明：系统设置中的 KYC_PRICE / KYC_PERSONAL_PRICE / KYC_ENTERPRISE_PRICE 对应产品库旧键
--       kyc_price / kyc_personal_price / kyc_enterprise_price；搬回后删除系统设置中的这三行。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL/DML 一律用反引号全限定库名.表名。

SET @has_dst = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'product_config');
SET @has_src = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');

SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT IGNORE INTO `oem_fv`.`product_config` (config_key, config_value, remark, created_at, updated_at)
     SELECT ''kyc_price'', s.config_value, ''平台 KYC 认证单价（元/次）'', NOW(), NOW()
       FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_PRICE''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT IGNORE INTO `oem_fv`.`product_config` (config_key, config_value, remark, created_at, updated_at)
     SELECT ''kyc_personal_price'', s.config_value, ''个人实名免费次数用尽后单价（元/次）'', NOW(), NOW()
       FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_PERSONAL_PRICE''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT IGNORE INTO `oem_fv`.`product_config` (config_key, config_value, remark, created_at, updated_at)
     SELECT ''kyc_enterprise_price'', s.config_value, ''企业实名免费次数用尽后单价（元/次）'', NOW(), NOW()
       FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_ENTERPRISE_PRICE''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(@has_src > 0,
    'DELETE FROM `oem_sys`.`setting`
      WHERE config_key IN (''KYC_PRICE'', ''KYC_PERSONAL_PRICE'', ''KYC_ENTERPRISE_PRICE'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;