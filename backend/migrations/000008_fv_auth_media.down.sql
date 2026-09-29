-- 000008 回滚：删除 auth_order 的身份要素与媒体保存字段
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order');

SET @has_media_expire = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'media_expire_at');
SET @sql_media_expire = IF(@has_tbl > 0 AND @has_media_expire > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `media_expire_at`',
    'SELECT 1');
PREPARE stmt_media_expire FROM @sql_media_expire; EXECUTE stmt_media_expire; DEALLOCATE PREPARE stmt_media_expire;

SET @has_media_dir = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'media_dir');
SET @sql_media_dir = IF(@has_tbl > 0 AND @has_media_dir > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `media_dir`',
    'SELECT 1');
PREPARE stmt_media_dir FROM @sql_media_dir; EXECUTE stmt_media_dir; DEALLOCATE PREPARE stmt_media_dir;

SET @has_id_card = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'id_card');
SET @sql_id_card = IF(@has_tbl > 0 AND @has_id_card > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `id_card`',
    'SELECT 1');
PREPARE stmt_id_card FROM @sql_id_card; EXECUTE stmt_id_card; DEALLOCATE PREPARE stmt_id_card;

SET @has_name = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'name');
SET @sql_name = IF(@has_tbl > 0 AND @has_name > 0,
    'ALTER TABLE `oem_fv`.`auth_order` DROP COLUMN `name`',
    'SELECT 1');
PREPARE stmt_name FROM @sql_name; EXECUTE stmt_name; DEALLOCATE PREPARE stmt_name;
