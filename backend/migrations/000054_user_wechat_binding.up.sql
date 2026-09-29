-- 000054 用户表新增微信绑定列（公众号网页授权 / 开放平台扫码登录）
-- 背景：微信一键登录仅用于已注册账号的快捷登录，绑定关系直接落在 user 表，不另开绑定表。
-- 说明：公众号与开放平台网站应用各自返回的 openid 不同，故分列存放；unionid 为同一开放平台账号下的统一标识，
--       公众号与网站应用未绑同一开放平台账号时微信不返回（可为空）。
-- 三列均为「可空 + 唯一索引」：MySQL 唯一索引不对 NULL 去重，故多行未绑定互不冲突；
-- 因此解绑必须写 NULL 而非空串（空串会被唯一索引判为冲突，导致第二个用户解绑失败）。
-- 禁止在迁移内 USE 切换会话库；跨库 DDL 一律反引号全限定库名.表名。幂等：已存在时跳过。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'wechat_unionid');
SET @ddl = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`user`
        ADD COLUMN `wechat_unionid` varchar(64) NULL COMMENT ''微信 unionid（同一开放平台账号下唯一，可能为空）'',
        ADD COLUMN `wechat_mp_openid` varchar(64) NULL COMMENT ''公众号网页授权 openid（手机端一键登录）'',
        ADD COLUMN `wechat_open_openid` varchar(64) NULL COMMENT ''开放平台网站应用 openid（PC 扫码登录）'',
        ADD COLUMN `wechat_nickname` varchar(64) NULL COMMENT ''微信昵称（扫码登录时取回，可为空）'',
        ADD UNIQUE KEY `uk_user_wechat_unionid` (`wechat_unionid`),
        ADD UNIQUE KEY `uk_user_wechat_mp_openid` (`wechat_mp_openid`),
        ADD UNIQUE KEY `uk_user_wechat_open_openid` (`wechat_open_openid`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;