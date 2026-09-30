-- 000055 移除微信开放平台登录残留（PC 扫码登录改为「服务号网页授权 + 本站在线二维码」）
-- 背景：PC 端不再使用微信开放平台网站应用（qrconnect / snsapi_login），与手机端共用公众号 openid
--       （wechat_mp_openid），user.wechat_open_openid 及其唯一索引不再使用；后台历史配置行
--       WECHAT_LOGIN_OPEN_APP_ID 一并清理（配置键与 .env 中的 WECHAT_LOGIN_OPEN_APP_SECRET 已随代码删除）。
-- 影响：历史仅通过开放平台 PC 扫码绑定过的账号，该绑定关系不再保留；若公众号已绑定同一开放平台账号，
--       网页授权会返回同一 unionid，扫码登录仍可命中并自动回填 wechat_mp_openid。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律反引号全限定库名.表名。幂等（索引/列/配置行不存在时跳过）。

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND INDEX_NAME = 'uk_user_wechat_open_openid');
SET @ddl_drop_idx = IF(@has_idx > 0,
    'ALTER TABLE `oem_sys`.`user` DROP INDEX `uk_user_wechat_open_openid`',
    'SELECT 1');
PREPARE stmt_drop_idx FROM @ddl_drop_idx; EXECUTE stmt_drop_idx; DEALLOCATE PREPARE stmt_drop_idx;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'wechat_open_openid');
SET @ddl_drop_col = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user` DROP COLUMN `wechat_open_openid`',
    'SELECT 1');
PREPARE stmt_drop_col FROM @ddl_drop_col; EXECUTE stmt_drop_col; DEALLOCATE PREPARE stmt_drop_col;

SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'setting');
SET @ddl_del_cfg = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`setting` WHERE config_key = ''WECHAT_LOGIN_OPEN_APP_ID''',
    'SELECT 1');
PREPARE stmt_del_cfg FROM @ddl_del_cfg; EXECUTE stmt_del_cfg; DEALLOCATE PREPARE stmt_del_cfg;