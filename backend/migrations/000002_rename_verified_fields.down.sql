-- 000002 down：将 verified_name/verified_number 改回 kyc_name/kyc_number

SET @has_verified_name = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'verified_name');
SET @sql = IF(@has_verified_name > 0,
    'ALTER TABLE `oem_sys`.`user` RENAME COLUMN `verified_name` TO `kyc_name`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_verified_number = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'user' AND COLUMN_NAME = 'verified_number');
SET @sql = IF(@has_verified_number > 0,
    'ALTER TABLE `oem_sys`.`user` RENAME COLUMN `verified_number` TO `kyc_number`',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
