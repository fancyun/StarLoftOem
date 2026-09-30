-- 000055 回滚：恢复微信开放平台登录所需的列、唯一索引与后台配置行
-- 说明：仅恢复结构与配置行，历史绑定数据无法恢复。幂等（已存在时跳过）。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'wechat_open_openid');
SET @ddl_add_col = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`user`
        ADD COLUMN `wechat_open_openid` varchar(64) NULL COMMENT ''开放平台网站应用 openid（PC 扫码登录）'' AFTER `wechat_mp_openid`',
    'SELECT 1');
PREPARE stmt_add_col FROM @ddl_add_col; EXECUTE stmt_add_col; DEALLOCATE PREPARE stmt_add_col;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'uk_user_wechat_open_openid');
SET @ddl_add_idx = IF(@has_idx = 0,
    'ALTER TABLE `oem_sys`.`user` ADD UNIQUE KEY `uk_user_wechat_open_openid` (`wechat_open_openid`)',
    'SELECT 1');
PREPARE stmt_add_idx FROM @ddl_add_idx; EXECUTE stmt_add_idx; DEALLOCATE PREPARE stmt_add_idx;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');
SET @ddl_add_cfg = IF(@has_tbl > 0,
    'INSERT IGNORE INTO `oem_sys`.`setting` (config_key, config_value, category, remark)
        VALUES (''WECHAT_LOGIN_OPEN_APP_ID'', '''', ''wechat'', ''开放平台网站应用 AppID（PC 扫码登录；留空表示 PC 端不可用）'')',
    'SELECT 1');
PREPARE stmt_add_cfg FROM @ddl_add_cfg; EXECUTE stmt_add_cfg; DEALLOCATE PREPARE stmt_add_cfg;