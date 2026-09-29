-- 000025 清理历史资源包扣量账单：bill 只记资金变动
-- 背景：资源包扣量（spend_type=pack_consume）余额前后相等、没有任何资金流动，
--       现已不再写入 bill；资源包消耗的条数/次数记在产品库业务表
--       （oem_sms.sms_send_record.pack_count / oem_fv.auth_record.pack_count）。
-- 本迁移删除历史扣量行；为避免误删，仅删除「余额前后一致」的行
-- （若某行余额确有变动，说明它承载了资金变动，保留待人工核对）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'bill');

SET @sql_del = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`bill`
      WHERE spend_type = ''pack_consume'' AND balance_before = balance_after',
    'SELECT 1');
PREPARE stmt_del FROM @sql_del; EXECUTE stmt_del; DEALLOCATE PREPARE stmt_del;