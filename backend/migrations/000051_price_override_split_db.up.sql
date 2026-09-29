-- 单用户定向定价（price_override）按产品分库：
--   人脸核验 fv_auth / fv_self → `oem_fv`.`price_override`
--   短信 sms                   → `oem_sms`.`price_override`
--   账户实名两档 kyc_personal / kyc_enterprise 仍留 `oem_sys`.`price_override`
-- 背景：原先一张系统库表承载 5 个服务的用户级定价，现按产品拆到各自产品库；
--       后台维护入口随之拆到「人脸核验 / 短信 → 产品配置」，用户管理仅保留实名两档。
-- 幂等：建表前判存在；搬迁不搬 id 且用 ON DUPLICATE KEY UPDATE，失败可重跑自愈；
--       删源行谓词与搬迁谓词逐字一致，只删已搬迁的产品行，账户实名两档不受影响。

-- ============================================================
-- 1. 在两张产品库建同构表（结构同 oem_sys.price_override）
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_fv`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''固定为 user-指定用户定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识；pack 时为「产品标识:资源包ID」'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT ''价格覆盖（单用户，人脸核验）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sms`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''固定为 user-指定用户定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识；pack 时为「产品标识:资源包ID」'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT ''价格覆盖（单用户，短信）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 2. 搬迁存量（人脸核验 / 短信）
-- ============================================================
-- 源表是否存在（全新环境无该表则整体跳过）
SET @src = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');

-- 2.1 人脸核验：unit 的 target 为 fv_auth / fv_self；pack 的 target 为 fv_auth:包ID / fv_self:包ID
--     （_ 在 LIKE 中是通配符，故用 LEFT 精确前缀匹配）
SET @sql = IF(@src > 0,
    'INSERT INTO `oem_fv`.`price_override` (scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT scope_type, scope_id, price_type, target, price, created_at, updated_at
       FROM `oem_sys`.`price_override`
      WHERE (price_type = ''unit'' AND target IN (''fv_auth'', ''fv_self''))
         OR (price_type = ''pack'' AND LEFT(target, 8) IN (''fv_auth:'', ''fv_self:''))
     ON DUPLICATE KEY UPDATE price = VALUES(price), updated_at = VALUES(updated_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2.2 短信：unit 的 target 为 sms；pack 的 target 为 sms:包ID
SET @sql = IF(@src > 0,
    'INSERT INTO `oem_sms`.`price_override` (scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT scope_type, scope_id, price_type, target, price, created_at, updated_at
       FROM `oem_sys`.`price_override`
      WHERE (price_type = ''unit'' AND target = ''sms'')
         OR (price_type = ''pack'' AND LEFT(target, 4) = ''sms:'')
     ON DUPLICATE KEY UPDATE price = VALUES(price), updated_at = VALUES(updated_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 3. 删除系统库中已搬迁的产品行（谓词与第 2 步一致；账户实名两档保留）
-- ============================================================
SET @sql = IF(@src > 0,
    'DELETE FROM `oem_sys`.`price_override`
      WHERE (price_type = ''unit'' AND target IN (''fv_auth'', ''fv_self'', ''sms''))
         OR (price_type = ''pack'' AND (LEFT(target, 8) IN (''fv_auth:'', ''fv_self:'') OR LEFT(target, 4) = ''sms:''))',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
