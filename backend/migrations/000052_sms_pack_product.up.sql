-- 000052 短信资源包：新增 product 类型列，区分「验证码/通知短信」与「营销短信」两类互不通用的资源包
-- 背景：短信资源包此前为单一产品（无类型维度），现并列营销短信资源包，实现方式与人脸核验的「有源/无源」一致：
--       resource_pack.product（定义）与 user_resource_pack.product（购买快照）取值 sms / sms_marketing，
--       扣费按「所发模板的类型」匹配对应类型的资源包，两类互不通用。
-- 处理：① 两张表各新增 product（默认 'sms'）；② 存量行回填 'sms'（历史只有验证码/通知一类）；
--       ③ 各建 product 索引（消费端按 user_id + product 匹配）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（列/索引存在性判断）。

-- ① 新增 product（资源包定义）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'resource_pack' AND COLUMN_NAME = 'product');
SET @sql_add_pack = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`resource_pack`
        ADD COLUMN `product` varchar(32) NOT NULL DEFAULT ''sms'' COMMENT ''所属类型：sms-验证码/通知短信、sms_marketing-营销短信'' AFTER `status`',
    'SELECT 1');
PREPARE stmt_add_pack FROM @sql_add_pack; EXECUTE stmt_add_pack; DEALLOCATE PREPARE stmt_add_pack;

-- ① 新增 product（用户已购资源包）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'product');
SET @sql_add_user_pack = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack`
        ADD COLUMN `product` varchar(32) NOT NULL DEFAULT ''sms'' COMMENT ''所属类型快照：sms-验证码/通知短信、sms_marketing-营销短信'' AFTER `remaining_count`',
    'SELECT 1');
PREPARE stmt_add_user_pack FROM @sql_add_user_pack; EXECUTE stmt_add_user_pack; DEALLOCATE PREPARE stmt_add_user_pack;

-- ② 存量行回填（历史资源包均属验证码/通知一类）
UPDATE `oem_sms`.`resource_pack` SET `product` = 'sms' WHERE COALESCE(`product`, '') = '';
UPDATE `oem_sms`.`user_resource_pack` SET `product` = 'sms' WHERE COALESCE(`product`, '') = '';

-- ③ 建索引（消费端按 user_id + product + status + remaining_count 匹配）
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'resource_pack' AND INDEX_NAME = 'idx_product');
SET @sql_idx_pack = IF(@has_idx = 0,
    'ALTER TABLE `oem_sms`.`resource_pack` ADD INDEX `idx_product` (`product`)',
    'SELECT 1');
PREPARE stmt_idx_pack FROM @sql_idx_pack; EXECUTE stmt_idx_pack; DEALLOCATE PREPARE stmt_idx_pack;

SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND INDEX_NAME = 'idx_product');
SET @sql_idx_user_pack = IF(@has_idx = 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` ADD INDEX `idx_product` (`product`)',
    'SELECT 1');
PREPARE stmt_idx_user_pack FROM @sql_idx_user_pack; EXECUTE stmt_idx_user_pack; DEALLOCATE PREPARE stmt_idx_user_pack;