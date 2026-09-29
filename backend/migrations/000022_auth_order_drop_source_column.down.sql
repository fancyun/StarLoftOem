-- 000022 回滚：恢复 auth_order.source 列（历史行为 2-API 调用，默认 2）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'source');

SET @sql_add = IF(@has_col = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `source` tinyint NOT NULL DEFAULT 2 COMMENT ''来源：2-API调用'' AFTER `cost`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;