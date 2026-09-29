-- 000029 账户实名单价从人脸核验产品配置迁至系统配置
-- 背景：kyc_price / kyc_personal_price / kyc_enterprise_price 属平台账户能力（账户实名）的定价，
--       此前随「人脸核验产品配置」预置进了 oem_fv.product_config，应归入系统库设置表
--       oem_sys.setting（后台「平台管理 → 系统设置」，分组 kyc），键名与 .env 保持一致。
--       注：kyc_price 现仅作「有源/无源人脸核验单价未配置时的兜底价」，人脸核验产品自身单价
--       为 fv_auth_price / fv_self_price（保留在人脸核验产品配置）。
-- 幂等：目标键已存在则不覆盖，最后删除产品库中的旧行。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL/DML 一律用反引号全限定库名.表名。

-- 1. 迁移 kyc_price → KYC_PRICE
SET @has_src = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'product_config');
SET @has_dst = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');

SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
     SELECT ''KYC_PRICE'', pc.config_value, ''kyc'', ''有源/无源人脸核验单价未配置时的兜底单价（元/次）'', NOW(), NOW()
       FROM `oem_fv`.`product_config` pc
      WHERE pc.config_key = ''kyc_price''
        AND NOT EXISTS (SELECT 1 FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_PRICE'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. 迁移 kyc_personal_price → KYC_PERSONAL_PRICE
SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
     SELECT ''KYC_PERSONAL_PRICE'', pc.config_value, ''kyc'', ''个人实名免费次数用尽后单价（元/次）'', NOW(), NOW()
       FROM `oem_fv`.`product_config` pc
      WHERE pc.config_key = ''kyc_personal_price''
        AND NOT EXISTS (SELECT 1 FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_PERSONAL_PRICE'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. 迁移 kyc_enterprise_price → KYC_ENTERPRISE_PRICE
SET @sql = IF(@has_src > 0 AND @has_dst > 0,
    'INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
     SELECT ''KYC_ENTERPRISE_PRICE'', pc.config_value, ''kyc'', ''企业实名免费次数用尽后单价（元/次）'', NOW(), NOW()
       FROM `oem_fv`.`product_config` pc
      WHERE pc.config_key = ''kyc_enterprise_price''
        AND NOT EXISTS (SELECT 1 FROM `oem_sys`.`setting` s WHERE s.config_key = ''KYC_ENTERPRISE_PRICE'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. 删除产品库中的旧行（键已改由系统配置管理）
SET @sql = IF(@has_src > 0,
    'DELETE FROM `oem_fv`.`product_config`
      WHERE config_key IN (''kyc_price'', ''kyc_personal_price'', ''kyc_enterprise_price'')',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;