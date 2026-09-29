-- 000008 FV 认证订单新增身份要素与媒体保存字段：
--   name / id_card：记录用户调用的姓名与身份证号（管理后台订单列表展示）
--   media_dir / media_expire_at：认证结果确定后自动下载的照片/视频保存目录与过期时间（30 天）
-- 全新库表结构由 AutoMigrate 补齐（此时表不存在则跳过）；存量库按信息架构检测补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order');

SET @has_name = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'name');
SET @sql_name = IF(@has_tbl > 0 AND @has_name = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `name` varchar(50) NOT NULL DEFAULT '''' COMMENT ''实名姓名（下游 API 调用时记录）'' AFTER `user_id`',
    'SELECT 1');
PREPARE stmt_name FROM @sql_name; EXECUTE stmt_name; DEALLOCATE PREPARE stmt_name;

SET @has_id_card = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'id_card');
SET @sql_id_card = IF(@has_tbl > 0 AND @has_id_card = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `id_card` varchar(18) NOT NULL DEFAULT '''' COMMENT ''实名身份证号（下游 API 调用时记录）'' AFTER `name`',
    'SELECT 1');
PREPARE stmt_id_card FROM @sql_id_card; EXECUTE stmt_id_card; DEALLOCATE PREPARE stmt_id_card;

SET @has_media_dir = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'media_dir');
SET @sql_media_dir = IF(@has_tbl > 0 AND @has_media_dir = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `media_dir` varchar(255) NOT NULL DEFAULT '''' COMMENT ''认证照片/视频本地保存目录（相对 UploadDir）'' AFTER `best_img_fetched`',
    'SELECT 1');
PREPARE stmt_media_dir FROM @sql_media_dir; EXECUTE stmt_media_dir; DEALLOCATE PREPARE stmt_media_dir;

SET @has_media_expire = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_order' AND COLUMN_NAME = 'media_expire_at');
SET @sql_media_expire = IF(@has_tbl > 0 AND @has_media_expire = 0,
    'ALTER TABLE `oem_fv`.`auth_order`
        ADD COLUMN `media_expire_at` datetime NULL DEFAULT NULL COMMENT ''认证照片/视频过期时间（完成时间+30天，过后清理）'' AFTER `media_dir`',
    'SELECT 1');
PREPARE stmt_media_expire FROM @sql_media_expire; EXECUTE stmt_media_expire; DEALLOCATE PREPARE stmt_media_expire;
