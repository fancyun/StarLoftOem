-- 000042 推广商推介方身份：affiliate 新增 referrer_type / referrer_id，id 改为代理自增
-- 背景：员工（admin_user）也可作为销售进行推广，而员工 id 与 user.id 不在同一编号空间，
--       原先「affiliate.id = 推广商 user_id」的做法会产生主键冲突，故改为显式记录推介方类型与 id。
-- 处理：① 新增 referrer_type（user-用户 / staff-员工）与 referrer_id 两列；
--       ② 存量行回填 referrer_type='user'、referrer_id=id（id 值保持不变，user.oem_id / aff_commission.oem_id 等引用继续有效）；
--       ③ id 改为代理自增（新建推广商不再显式指定 id）；
--       ④ 加唯一键 (referrer_type, referrer_id)，保证同一推介方只有一个推广商身份。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（列/索引存在性判断）。

-- ① 新增 referrer_type
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'referrer_type');
SET @sql_add_type = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`affiliate`
        ADD COLUMN `referrer_type` varchar(8) NOT NULL DEFAULT ''user'' COMMENT ''推介方类型：user-用户 staff-员工'' AFTER `id`',
    'SELECT 1');
PREPARE stmt_add_type FROM @sql_add_type; EXECUTE stmt_add_type; DEALLOCATE PREPARE stmt_add_type;

-- ① 新增 referrer_id
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'referrer_id');
SET @sql_add_id = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`affiliate`
        ADD COLUMN `referrer_id` bigint NOT NULL DEFAULT 0 COMMENT ''推介方 ID（user 类型为 user.id，staff 类型为 admin_user.id）'' AFTER `referrer_type`',
    'SELECT 1');
PREPARE stmt_add_id FROM @sql_add_id; EXECUTE stmt_add_id; DEALLOCATE PREPARE stmt_add_id;

-- ② 存量行回填：原有推广商均由用户申请，referrer_id 即其 user_id（当前 id 值）
UPDATE `oem_sys`.`affiliate` SET `referrer_id` = `id` WHERE `referrer_id` = 0;

-- ③ id 改代理自增（已自增则跳过）
SET @is_auto = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'id'
      AND EXTRA LIKE '%auto_increment%');
SET @sql_auto = IF(@is_auto = 0,
    'ALTER TABLE `oem_sys`.`affiliate` MODIFY COLUMN `id` bigint NOT NULL AUTO_INCREMENT',
    'SELECT 1');
PREPARE stmt_auto FROM @sql_auto; EXECUTE stmt_auto; DEALLOCATE PREPARE stmt_auto;

-- ④ 唯一键：同一推介方仅一个推广商身份（已存在则跳过）
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND INDEX_NAME = 'uk_affiliate_referrer');
SET @sql_idx = IF(@has_idx = 0,
    'ALTER TABLE `oem_sys`.`affiliate` ADD UNIQUE KEY `uk_affiliate_referrer` (`referrer_type`, `referrer_id`)',
    'SELECT 1');
PREPARE stmt_idx FROM @sql_idx; EXECUTE stmt_idx; DEALLOCATE PREPARE stmt_idx;