-- 000018 认证订单新增 token_expire_at（上游 get_token 返回的 expired_time，链接有效期）
-- 背景：发起核验对外返回的链接有效期原为硬编码「创建时间 + 15 分钟」，与上游真实有效期不一致；
--       改为落库上游返回值，由发起接口按实际上游有效期返回 expired_time / expired_in，承接页倒计时同源。
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'token_expire_at');
SET @sql_add = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `token_expire_at` datetime NULL DEFAULT NULL COMMENT ''上游 token 到期时间（get_token 返回的 expired_time）'' AFTER `up_request_id`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;