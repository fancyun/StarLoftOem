-- 000013 新建用户上传文件登记表 upload_file，并回填存量短信签名资质图归属
-- 背景：营业执照/身份证等资质图受 UploadAuth 保护，控制台用户需读取本人上传的图，
--       故按「上传者 + 相对路径」登记归属，鉴权时按当前登录用户校验归属放行。
-- 图片按内容 MD5 命名去重：同一张图被不同用户上传时各登记一行。
-- 本迁移显式建表（存放于系统库，已存在则跳过），AutoMigrate 后续按模型兜底补列。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'upload_file');

SET @sql_create = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`upload_file` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `user_id` bigint NOT NULL DEFAULT 0 COMMENT ''上传用户'',
        `file_path` varchar(255) NOT NULL COMMENT ''上传目录下相对路径 yyyyMMdd/{md5}.jpg'',
        `file_url` varchar(512) NOT NULL DEFAULT '''' COMMENT ''对外图片地址'',
        `file_size` bigint NOT NULL DEFAULT 0 COMMENT ''字节数'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_user_path` (`user_id`, `file_path`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''用户上传文件登记''',
    'SELECT 1');
PREPARE stmt_create FROM @sql_create; EXECUTE stmt_create; DEALLOCATE PREPARE stmt_create;

-- 存量回填：sms_sign 中各资质图按提交用户登记归属（同一张图被同一用户多次使用时由 DISTINCT 去重）
-- 仅在登记表为空时执行（一次性回填，天然幂等，且避免自引用子查询）；
-- 新建库中 sms_sign 尚未由 AutoMigrate 建出（迁移先于 AutoMigrate 执行），此时跳过
SET @has_src = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sms' AND TABLE_NAME = 'sms_sign');
SET @has_row = (SELECT COUNT(*) FROM `oem_sys`.`upload_file`);

SET @sql_backfill = IF(@has_src > 0 AND @has_row = 0,
    'INSERT INTO `oem_sys`.`upload_file` (user_id, file_path, file_url, file_size, created_at)
     SELECT DISTINCT s.user_id, SUBSTRING_INDEX(t.url, ''/uploads/'', -1), t.url, 0, NOW()
       FROM `oem_sms`.`sms_sign` s
       JOIN (
             SELECT id, credit_code_url AS url FROM `oem_sms`.`sms_sign`
       UNION ALL SELECT id, id_card_front FROM `oem_sms`.`sms_sign`
       UNION ALL SELECT id, id_card_back FROM `oem_sms`.`sms_sign`
       UNION ALL SELECT id, sx_commits FROM `oem_sms`.`sms_sign`
       UNION ALL SELECT id, auth_letter FROM `oem_sms`.`sms_sign`
       UNION ALL SELECT id, screenshot FROM `oem_sms`.`sms_sign`
            ) t ON t.id = s.id
      WHERE t.url LIKE ''%/uploads/%''',
    'SELECT 1');
PREPARE stmt_backfill FROM @sql_backfill; EXECUTE stmt_backfill; DEALLOCATE PREPARE stmt_backfill;