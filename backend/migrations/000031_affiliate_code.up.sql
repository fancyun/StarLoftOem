-- 000031 推广商新增推广码 aff_code（12 位，数字 + 小写字母）
-- 用途：推广链接 {console}/register?ref={aff_code}，注册时按码绑定归属（取代原先仅按域名判定）。
-- 说明：
--   1) 列先允许 NULL，为存量推广商回填后再加唯一索引（避免多行默认空串冲突）；
--   2) 存量码取「id 转 36 进制补足 6 位 + MD5 随机段 6 位」，字符集恒为 0-9a-z，共 12 位；
--   3) 新推广商的码由应用层生成（crypto/rand，查重保证唯一）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1. 加列（幂等）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'aff_code');
SET @sql = IF(@has_col = 0,
    'ALTER TABLE `oem_sys`.`affiliate`
        ADD COLUMN `aff_code` varchar(12) NULL COMMENT ''推广码（12 位数字+小写字母，用于推广链接 ?ref='' AFTER `domain`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. 为存量行回填（仅补空值，幂等）
UPDATE `oem_sys`.`affiliate`
SET `aff_code` = LOWER(CONCAT(
        SUBSTRING(LPAD(CONV(`id`, 10, 36), 6, '0'), 1, 6),
        SUBSTRING(MD5(CONCAT(`id`, ':', RAND(), ':', NOW(6))), 1, 6)
    ))
WHERE `aff_code` IS NULL OR `aff_code` = '';

-- 3. 唯一索引（幂等）
SET @has_idx = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND INDEX_NAME = 'uk_affiliate_code');
SET @sql = IF(@has_idx = 0,
    'ALTER TABLE `oem_sys`.`affiliate` ADD UNIQUE KEY `uk_affiliate_code` (`aff_code`)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
