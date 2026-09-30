-- 000057 通知重试记录改以「业务单号」标识（复合主键 biz_type + biz_no，去掉自增 id）
-- 背景：notify_record 原以自增 id 为主键，与业务无关、不能按业务单号直接定位；且同一业务单号反复失败会插入多行。
--       改为复合主键 (biz_type, biz_no)：同一业务单号只保留一行（重新入队即重置重试），
--       且推送成功后直接删除该行（成功历史不再落表，留痕见 syscall.log / business.log）。
-- 数据安全（不丢行）：biz_no 先以可空列加入并回填——
--       fv_result  → auth_record.biz_no
--       sms_receipt→ sms_send_record.biz_no
--       sms_reply  → sms_reply.sequence_id
--       其余（sms_status 等，无法反查业务单号）→ legacy-{id}，保留该行与重推能力；
--       随后按 (biz_type, biz_no) 去重（保留最新一行），最后删除 id 列并建立复合主键。
-- 说明：禁止在迁移内 USE 切换会话库；跨库 DDL 一律反引号全限定库名.表名。
--       幂等：各步均先判断是否已执行；表不存在（全新库，表结构由 AutoMigrate 在迁移之后补齐）时整体跳过。

-- 0) 表是否已存在（全新库为 0，此时结构由 AutoMigrate 直接按模型创建，无需本迁移改造）
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record');

-- 1) 增加 biz_no 列（可空）
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND COLUMN_NAME = 'biz_no');
SET @ddl = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_sys`.`notify_record` ADD COLUMN `biz_no` varchar(64) NULL AFTER `biz_type`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2) 回填业务单号（仅处理尚无值的行）
SET @has_id = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND COLUMN_NAME = 'id');
SET @ddl = IF(@has_id > 0,
    'UPDATE `oem_sys`.`notify_record` n SET n.biz_no = COALESCE(
        (SELECT a.biz_no FROM `oem_fv`.`auth_record` a WHERE n.biz_type = ''fv_result'' AND a.id = n.record_id),
        (SELECT s.biz_no FROM `oem_sms`.`sms_send_record` s WHERE n.biz_type = ''sms_receipt'' AND s.id = n.record_id),
        (SELECT r.sequence_id FROM `oem_sms`.`sms_reply` r WHERE n.biz_type = ''sms_reply'' AND r.id = n.record_id),
        CONCAT(''legacy-'', n.id))
      WHERE n.biz_no IS NULL OR n.biz_no = ''''',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3) 按 (biz_type, biz_no) 去重，保留最新一行
SET @ddl = IF(@has_id > 0,
    'DELETE t1 FROM `oem_sys`.`notify_record` t1
        JOIN `oem_sys`.`notify_record` t2
          ON t1.biz_type = t2.biz_type AND t1.biz_no = t2.biz_no AND t1.id < t2.id',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4) 去掉自增主键与 id 列（AUTO_INCREMENT 需先解除）
SET @ddl = IF(@has_id > 0,
    'ALTER TABLE `oem_sys`.`notify_record` MODIFY COLUMN `id` bigint NOT NULL',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_pk = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record'
      AND INDEX_NAME = 'PRIMARY' AND COLUMN_NAME = 'id');
SET @ddl = IF(@has_pk > 0,
    'ALTER TABLE `oem_sys`.`notify_record` DROP PRIMARY KEY, DROP COLUMN `id`',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record' AND COLUMN_NAME = 'biz_no');
SET @ddl = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`notify_record` MODIFY COLUMN `biz_no` varchar(64) NOT NULL',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_pk = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'notify_record'
      AND INDEX_NAME = 'PRIMARY' AND COLUMN_NAME = 'biz_no');
SET @ddl = IF(@has_tbl > 0 AND @has_pk = 0,
    'ALTER TABLE `oem_sys`.`notify_record` ADD PRIMARY KEY (`biz_type`, `biz_no`)',
    'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;