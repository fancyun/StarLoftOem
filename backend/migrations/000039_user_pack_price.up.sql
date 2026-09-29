-- 000039 用户已购资源包：去掉冗余的 pack_id，改记购买价格快照（名称/数量/价格即可）
-- 背景：user_resource_pack 仅用于展示与按次扣减——消费匹配按 user_id + product + status + remaining_count，
--       退款按 user_resource_pack.id（业务表的 user_pack_id / pack_id），均不依赖资源包定义 ID；
--       pack_name / total_count 已是购买时快照，本次补 price（实付金额，元）作为价格快照。
-- 处理：① 两库 user_resource_pack 新增 price；
--       ② 先用资源包定义售价按 pack_id 回填，再用账单实付覆盖（在线/组合支付 = 支付单金额 + 余额部分；余额支付 = 账单金额）；
--       ③ 删除 pack_id 列（该列历史值不再需要，回滚无法恢复其数据）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（列存在性判断）。

-- ① 新增 price（人脸核验库）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'price');
SET @sql_add_fv = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack`
        ADD COLUMN `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''购买价格（元，快照：实付金额）'' AFTER `product`',
    'SELECT 1');
PREPARE stmt_add_fv FROM @sql_add_fv; EXECUTE stmt_add_fv; DEALLOCATE PREPARE stmt_add_fv;

-- ① 新增 price（短信库）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack');
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'price');
SET @sql_add_sms = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack`
        ADD COLUMN `price` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''购买价格（元，快照：实付金额）'' AFTER `remaining_count`',
    'SELECT 1');
PREPARE stmt_add_sms FROM @sql_add_sms; EXECUTE stmt_add_sms; DEALLOCATE PREPARE stmt_add_sms;

-- ② 人脸核验：定义售价兜底（定义表已删除的行保持 0）
UPDATE `oem_fv`.`user_resource_pack` u
JOIN `oem_fv`.`resource_pack` p ON p.id = u.pack_id
SET u.price = p.price, u.updated_at = u.updated_at
WHERE COALESCE(u.price, 0) = 0;

-- ② 人脸核验：余额支付账单实付覆盖（用户定向定价可能与定义价不同）
UPDATE `oem_fv`.`user_resource_pack` u
JOIN `oem_sys`.`bill` b
  ON b.ref_type = 'user_resource_pack' AND b.ref_id = u.id AND b.spend_type = 'pack_purchase'
SET u.price = b.amount, u.updated_at = u.updated_at
WHERE COALESCE(b.pay_order_id, 0) = 0 AND COALESCE(b.amount, 0) > 0;

-- ② 人脸核验：在线/组合支付按支付单实付覆盖（第三方支付部分 + 余额支付部分）
UPDATE `oem_fv`.`user_resource_pack` u
JOIN `oem_sys`.`bill` b
  ON b.ref_type = 'user_resource_pack' AND b.ref_id = u.id AND b.spend_type = 'pack_purchase'
SET u.price = b.amount + COALESCE(b.balance_before, 0), u.updated_at = u.updated_at
WHERE COALESCE(b.pay_order_id, 0) > 0;

-- ② 短信：定义售价兜底
UPDATE `oem_sms`.`user_resource_pack` u
JOIN `oem_sms`.`resource_pack` p ON p.id = u.pack_id
SET u.price = p.price, u.updated_at = u.updated_at
WHERE COALESCE(u.price, 0) = 0;

-- ② 短信：账单实付覆盖
UPDATE `oem_sms`.`user_resource_pack` u
JOIN `oem_sys`.`bill` b
  ON b.ref_type = 'sms_user_resource_pack' AND b.ref_id = u.id AND b.spend_type = 'pack_purchase'
SET u.price = b.amount, u.updated_at = u.updated_at
WHERE COALESCE(b.amount, 0) > 0;

-- ③ 删除 pack_id（人脸核验库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_id');
SET @sql_drop_fv = IF(@has_col > 0,
    'ALTER TABLE `oem_fv`.`user_resource_pack` DROP COLUMN `pack_id`',
    'SELECT 1');
PREPARE stmt_drop_fv FROM @sql_drop_fv; EXECUTE stmt_drop_fv; DEALLOCATE PREPARE stmt_drop_fv;

-- ③ 删除 pack_id（短信库）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'user_resource_pack' AND COLUMN_NAME = 'pack_id');
SET @sql_drop_sms = IF(@has_col > 0,
    'ALTER TABLE `oem_sms`.`user_resource_pack` DROP COLUMN `pack_id`',
    'SELECT 1');
PREPARE stmt_drop_sms FROM @sql_drop_sms; EXECUTE stmt_drop_sms; DEALLOCATE PREPARE stmt_drop_sms;