-- 000028 回滚：把 OEM 分销表从独立库迁回系统库 oem_sys
-- 说明：出库流水表 oem_stock_log 为本次新增能力，回滚时直接删除（系统库无对应表）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1. 系统库建回原表（结构一致）

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

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'price_override');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`price_override` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `scope_type` varchar(16) NOT NULL COMMENT ''user-指定用户 oem-OEM 给下级定价'',
        `scope_id` bigint NOT NULL COMMENT ''scope_type=user 时为 user_id；=oem 时为 OEM 管理员 user_id'',
        `price_type` varchar(16) NOT NULL DEFAULT ''unit'' COMMENT ''unit-按次/按条单价 pack-资源包售价'',
        `target` varchar(64) NOT NULL COMMENT ''unit 时为服务标识；pack 时为「产品标识:资源包ID」'',
        `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''覆盖价（元）'',
        `created_at` datetime(3) NULL,
        `updated_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_price_scope` (`scope_type`,`scope_id`,`price_type`,`target`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''价格覆盖（OEM/单用户）''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'oem_settlement');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`oem_settlement` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''OEM 管理员 user_id（收款方）'',
        `user_id` bigint NOT NULL COMMENT ''结算来源用户 user_id（付款方）'',
        `amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''结算金额，正=结算给 OEM，负=退款冲回'',
        `biz_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''pack_purchase/sms_send/fv_auth/refund'',
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

-- 2. 数据迁回系统库（新库表存在 且 系统库表为空 时迁移一次）

SET @new_oem = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem');
SET @old_oem_rows = (SELECT COUNT(*) FROM `oem_sys`.`oem`);
SET @sql = IF(@new_oem > 0 AND @old_oem_rows = 0,
    'INSERT INTO `oem_sys`.`oem` (id, name, domain, contact, status, reject_reason, remark, created_at, updated_at)
     SELECT id, name, domain, contact, status, reject_reason, remark, created_at, updated_at FROM `starloft_oem`.`oem`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @new_po = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'price_override');
SET @old_po_rows = (SELECT COUNT(*) FROM `oem_sys`.`price_override`);
SET @sql = IF(@new_po > 0 AND @old_po_rows = 0,
    'INSERT INTO `oem_sys`.`price_override` (id, scope_type, scope_id, price_type, target, price, created_at, updated_at)
     SELECT id, scope_type, scope_id, price_type, target, price, created_at, updated_at FROM `starloft_oem`.`price_override`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @new_se = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'starloft_oem' AND TABLE_NAME = 'oem_settlement');
SET @old_se_rows = (SELECT COUNT(*) FROM `oem_sys`.`oem_settlement`);
SET @sql = IF(@new_se > 0 AND @old_se_rows = 0,
    'INSERT INTO `oem_sys`.`oem_settlement` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
     SELECT id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at FROM `starloft_oem`.`oem_settlement`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. 删除出库流水与新库
DROP TABLE IF EXISTS `starloft_oem`.`oem_stock_log`;
DROP TABLE IF EXISTS `starloft_oem`.`oem_settlement`;
DROP TABLE IF EXISTS `starloft_oem`.`price_override`;
DROP TABLE IF EXISTS `starloft_oem`.`oem`;
DROP DATABASE IF EXISTS `starloft_oem`;