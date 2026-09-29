-- 000021 回滚：恢复 kyc.source 列（历史行为 1-账户实名，默认 1）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc' AND COLUMN_NAME = 'source');

SET @sql_add = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`kyc`
        ADD COLUMN `source` tinyint NOT NULL DEFAULT 1 COMMENT ''记录来源：1-账户实名'' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;