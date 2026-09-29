-- 000024 认证记录命名统一：oem_fv.auth_order → auth_record
-- 背景：原「人脸核验认证订单」概念在代码中统一改称「认证记录」，表名、关联列与账单业务类型同步更名。
-- 步骤：
--   1) oem_fv.auth_order 表重命名为 auth_record（存量库旧表存在且新表不存在时执行）；
--   2) oem_sys.kyc 关联列 auth_order_id 重命名为 auth_record_id（含同名索引）；
--   3) oem_sys.bill 中 ref_type='auth_order' 的历史行更新为 ref_type='auth_record'。
-- 全新库此时表结构尚未由 AutoMigrate 创建，守卫检测会跳过，由 AutoMigrate 直接建新名表/列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1) FV 库认证记录表重命名
SET @has_old_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order');
SET @has_new_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_record');
SET @sql_tbl = IF(@has_old_tbl > 0 AND @has_new_tbl = 0,
    'RENAME TABLE `oem_fv`.`auth_order` TO `oem_fv`.`auth_record`',
    'SELECT 1');
PREPARE stmt_tbl FROM @sql_tbl; EXECUTE stmt_tbl; DEALLOCATE PREPARE stmt_tbl;

-- 2) 系统库 kyc 关联列重命名（auth_order_id → auth_record_id）及索引
SET @has_kyc = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc');
SET @has_old_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND COLUMN_NAME = 'auth_order_id');
SET @has_new_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND COLUMN_NAME = 'auth_record_id');
SET @sql_col = IF(@has_kyc > 0 AND @has_old_col > 0 AND @has_new_col = 0,
    'ALTER TABLE `oem_sys`.`kyc` CHANGE COLUMN `auth_order_id` `auth_record_id` bigint NULL',
    'SELECT 1');
PREPARE stmt_col FROM @sql_col; EXECUTE stmt_col; DEALLOCATE PREPARE stmt_col;

SET @has_old_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND INDEX_NAME = 'idx_kyc_auth_order_id');
SET @sql_idx = IF(@has_kyc > 0 AND @has_old_idx > 0,
    'ALTER TABLE `oem_sys`.`kyc` RENAME INDEX `idx_kyc_auth_order_id` TO `idx_kyc_auth_record_id`',
    'SELECT 1');
PREPARE stmt_idx FROM @sql_idx; EXECUTE stmt_idx; DEALLOCATE PREPARE stmt_idx;

-- 3) 统一账单 ref_type 历史值同步更名
SET @has_bill = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill');
SET @sql_bill = IF(@has_bill > 0,
    'UPDATE `oem_sys`.`bill` SET ref_type = ''auth_record'' WHERE ref_type = ''auth_order''',
    'SELECT 1');
PREPARE stmt_bill FROM @sql_bill; EXECUTE stmt_bill; DEALLOCATE PREPARE stmt_bill;