-- 000054 回滚：删除用户表微信绑定列（唯一索引随列一并删除）
-- 幂等：列不存在时跳过。禁止在迁移内 USE 切换会话库。

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'wechat_unionid');
SET @ddl = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`user`
        DROP COLUMN `wechat_unionid`,
        DROP COLUMN `wechat_mp_openid`,
        DROP COLUMN `wechat_open_openid`,
        DROP COLUMN `wechat_nickname`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;