-- 000020 实名记录表只保留账户实名：清理下游 API 调用（source=2）的历史行
-- 背景：oem_sys.kyc 原先混存「账户实名（source=1）」与「下游 API 调用（source=2）」两类记录，
--       而 API 调用的核验信息与结果已完整落在认证订单 oem_fv.auth_order
--       （name / id_card / up_token / up_biz_id / status / result_code / result_message / 时间，且两表通过 auth_order_id 关联）。
--       现改为：账户实名只写 kyc，下游 API 调用只走 auth_order，不再重复写实名记录。
-- 本迁移删除 source=2 且已关联认证订单的历史行（信息在 auth_order 中完整保留）；
-- 未关联订单的异常历史行保持原样，代码不再产生新的 source=2 记录。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND COLUMN_NAME = 'source');

SET @sql_del = IF(@has_tbl > 0 AND @has_col > 0,
    'DELETE FROM `oem_sys`.`kyc` WHERE source = 2 AND COALESCE(auth_order_id, 0) > 0',
    'SELECT 1');
PREPARE stmt_del FROM @sql_del; EXECUTE stmt_del; DEALLOCATE PREPARE stmt_del;