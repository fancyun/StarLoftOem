-- 000032 推广提现新增「提现方式」channel
-- 背景：提现默认改为「提现到平台余额」（站内划转，即时到账、免手续费、无需收款信息）；
--       支付宝/微信/银行卡等线下渠道后续扩展，届时才需要收款信息与人工审核。
-- 说明：存量记录按当时的口径视为线下提现，仍落 balance 默认值（不影响历史状态与金额）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'aff_withdraw' AND COLUMN_NAME = 'channel');
SET @sql = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`aff_withdraw`
        ADD COLUMN `channel` varchar(16) NOT NULL DEFAULT ''balance''
        COMMENT ''提现方式：balance-提现到余额（即时到账，后续扩展线下渠道）'' AFTER `oem_id`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
