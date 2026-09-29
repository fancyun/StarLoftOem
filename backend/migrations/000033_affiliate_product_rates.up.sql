-- 000033 推广商提成比例按产品分档（人脸核验 fv / 短信 sms）
-- 背景：原先 affiliate 只有单一 commission_rate，短信与人脸核验只能配同一个比例，
--       无法「短信多少、核验多少」分别设置。
-- 本迁移：新增 commission_rate_fv / commission_rate_sms 两列，并把存量行回填为改造前生效的比例，
--         保证改造前后计提金额完全一致。
-- 说明：
--   1) 两列为计提时的权威取值（0 表示该产品不提成）；
--      commission_rate 保留为「默认档」——新建推广商时作为两列的初值，不再参与计提。
--   2) 仅在本次新增列时回填，避免覆盖管理员后续单独调整过的档位。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 是否本次新增（用于决定是否回填存量）
SET @has_fv = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'commission_rate_fv');
SET @has_sms = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'affiliate' AND COLUMN_NAME = 'commission_rate_sms');
SET @need_backfill = IF(@has_fv = 0 OR @has_sms = 0, 1, 0);

-- 1. 加列（幂等）
SET @sql = IF(@has_fv = 0,
    'ALTER TABLE `oem_sys`.`affiliate`
        ADD COLUMN `commission_rate_fv` decimal(6,4) NOT NULL DEFAULT 0.2
        COMMENT ''人脸核验（fv）提成比例（0~1），0 表示该产品不提成'' AFTER `commission_rate`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(@has_sms = 0,
    'ALTER TABLE `oem_sys`.`affiliate`
        ADD COLUMN `commission_rate_sms` decimal(6,4) NOT NULL DEFAULT 0.2
        COMMENT ''短信（sms）提成比例（0~1），0 表示该产品不提成'' AFTER `commission_rate_fv`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. 存量回填：两档均取改造前生效的单一比例
SET @sql = IF(@need_backfill = 1,
    'UPDATE `oem_sys`.`affiliate`
        SET `commission_rate_fv` = `commission_rate`,
            `commission_rate_sms` = `commission_rate`,
            `updated_at` = NOW()',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
