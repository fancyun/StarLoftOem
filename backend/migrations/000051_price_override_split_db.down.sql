-- 逆向：把两张产品库的覆盖价并回系统库，再删除产品库表；
-- 账户实名两档本就在系统库，受影响为零。
-- 幂等：源表存在才搬迁；并回用 ON DUPLICATE KEY UPDATE（不搬 id）；DROP 一律 IF EXISTS。

-- ============================================================
-- 1. 系统库表兜底存在（正常不会缺，仅防人工误删）
-- ============================================================
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`price_override` (
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
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT ''价格覆盖（单用户）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 2. 两张产品库的数据并回系统库
-- ============================================================
SET @has_fv = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_fv > 0,
    'INSERT INTO `oem_sys`.`price_override` (scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT scope_type, scope_id, price_type, target, price, created_at, updated_at FROM `oem_fv`.`price_override`
     ON DUPLICATE KEY UPDATE price = VALUES(price), updated_at = VALUES(updated_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_sms = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_sms > 0,
    'INSERT INTO `oem_sys`.`price_override` (scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT scope_type, scope_id, price_type, target, price, created_at, updated_at FROM `oem_sms`.`price_override`
     ON DUPLICATE KEY UPDATE price = VALUES(price), updated_at = VALUES(updated_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 3. 删除两张产品库表
-- ============================================================
DROP TABLE IF EXISTS `oem_fv`.`price_override`;
DROP TABLE IF EXISTS `oem_sms`.`price_override`;
