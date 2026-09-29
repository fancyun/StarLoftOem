-- 000027 OEM 分销与配置体系改造
-- 背景：
--   1) OEM 分销：任意用户可申请成为 OEM（表 oem，主键即其 user_id），其下级用户按 user.oem_id 归属直接上级，
--      支持下级再申请 OEM（套娃），结算只发生在相邻两级。
--   2) 价格覆盖：统一用 price_override 承载「OEM 给下级定价」与「平台给单个用户定价」，解析链为
--      用户级 → OEM 链逐级 → 平台产品库价。
--   3) 配置体系改造：第三方密钥/业务配置迁入 oem_sys.setting，产品配置迁入对应产品库的 product_config。
-- 说明：表结构同时由 AutoMigrate 兜底补齐；本迁移显式建表以便存量库先行落地并保持幂等。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1. 系统配置表（密钥与第三方业务配置）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`setting` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `config_key` varchar(64) NOT NULL COMMENT ''配置键（如 ALIPAY_APP_ID）'',
        `config_value` text COMMENT ''配置值（密钥明文存储，仅后台管理员可见）'',
        `category` varchar(32) NOT NULL DEFAULT ''common'' COMMENT ''分组：finauth/tencent/alipay/wechat/ses/sms/jwt/security/common'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''说明'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_setting_key` (`config_key`),
        KEY `idx_setting_category` (`category`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''系统配置（密钥/第三方业务配置）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. 价格覆盖表（OEM 定价 / 单用户自定义单价）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''user-指定用户 oem-OEM 给下级定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id；=oem 时为 OEM 管理员 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识(fv_auth/fv_self/kyc_personal/kyc_enterprise/sms)；pack 时为资源包ID'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''价格覆盖（OEM/单用户）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. OEM 主体表（主键即 OEM 管理员 user_id）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'oem');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`oem` (
        `id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（与用户表主键一致）'',
        `name` varchar(100) NOT NULL DEFAULT '''' COMMENT ''OEM 品牌名'',
        `domain` varchar(190) NOT NULL DEFAULT '''' COMMENT ''自有域名（空表示未配置）'',
        `contact` varchar(100) NOT NULL DEFAULT '''' COMMENT ''联系方式'',
        `status` tinyint NOT NULL DEFAULT 0 COMMENT ''0-待审核 1-已通过 2-已驳回 3-已停用'',
        `reject_reason` varchar(255) NOT NULL DEFAULT '''' COMMENT ''驳回原因'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''平台备注'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_domain` (`domain`),
        KEY `idx_oem_status` (`status`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 分销主体''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. OEM 结算流水（下级消费/买包结算给直接上级）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'oem_settlement');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`oem_settlement` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（收款方）'',
        `user_id` bigint NOT NULL COMMENT ''结算来源用户 user_id（付款方）'',
        `amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''结算金额，正=结算给 OEM，负=退款冲回'',
        `biz_type` varchar(32) NOT NULL DEFAULT '' COMMENT ''pack_purchase/sms_send/fv_auth/refund'',
        `ref_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''关联业务表类型'',
        `ref_id` bigint NOT NULL DEFAULT 0 COMMENT ''关联业务主键'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''备注'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_oem_settle_oem` (`oem_id`,`created_at`),
        KEY `idx_oem_settle_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''OEM 结算流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 5. 产品配置表（人脸核验库 / 短信库）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'product_config');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_fv`.`product_config` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `config_key` varchar(64) NOT NULL COMMENT ''配置键（如 fv_auth_price）'',
        `config_value` varchar(512) NOT NULL DEFAULT '''' COMMENT ''配置值'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''说明'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_fv_config_key` (`config_key`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''人脸核验产品配置''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'product_config');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sms`.`product_config` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `config_key` varchar(64) NOT NULL COMMENT ''配置键（如 sms_price）'',
        `config_value` varchar(512) NOT NULL DEFAULT '''' COMMENT ''配置值'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''说明'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_sms_config_key` (`config_key`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''短信产品配置''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 6. 用户表新增 OEM 归属字段（0=平台直营；>0 为其直接上级 OEM 管理员 user_id）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'oem_id');
SET @sql = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`user`
        ADD COLUMN `oem_id` bigint NOT NULL DEFAULT 0 COMMENT ''所属 OEM（=直接上级管理员 user_id，0=平台直营）'',
        ADD KEY `idx_user_oem` (`oem_id`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;