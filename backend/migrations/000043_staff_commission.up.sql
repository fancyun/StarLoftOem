-- 000043 员工销售提成独立成表：员工提成由 aff_commission 迁出到 staff_commission
-- 背景：员工销售与用户型推广商（aff）是两类不同业务——提成口径与有效期不同
--       （aff 仅归因被推广用户注册后 1 个月内的消费，员工销售长期有效），
--       故提成流水分表存放，避免两类业务在同一张表里混淆与互相影响。
-- 处理：① 新建 staff_commission（结构与 aff_commission 一致）；
--       ② 将 affiliate.referrer_type='staff' 的提成行按原 id 迁移到新表；
--       ③ 从 aff_commission 删除这些行（天然幂等：删除后再次执行无可迁移行）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。

-- ① 新建员工销售提成流水表
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'staff_commission');
SET @sql = IF(@has_tbl = 0,
    'CREATE TABLE `oem_sys`.`staff_commission` (
        `id` bigint NOT NULL AUTO_INCREMENT,
        `oem_id` bigint NOT NULL COMMENT ''员工销售 affiliate.id（收款方）'',
        `user_id` bigint NOT NULL COMMENT ''提成来源用户 user_id'',
        `amount` decimal(10,2) NOT NULL DEFAULT 0 COMMENT ''提成金额，正=计提，负=退款冲回'',
        `biz_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''pack_purchase/sms_send/fv_auth/refund'',
        `ref_type` varchar(32) NOT NULL DEFAULT '''' COMMENT ''关联业务表类型'',
        `ref_id` bigint NOT NULL DEFAULT 0 COMMENT ''关联业务主键'',
        `remark` varchar(255) NOT NULL DEFAULT '''' COMMENT ''备注'',
        `created_at` datetime(3) NULL,
        PRIMARY KEY (`id`),
        KEY `idx_staff_settle_oem` (`oem_id`,`created_at`),
        KEY `idx_staff_settle_user` (`user_id`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT ''员工销售提成流水''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ② 迁移员工型提成行到新表（保留原 id，便于按单号追溯）
INSERT INTO `oem_sys`.`staff_commission` (id, oem_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
SELECT c.id, c.oem_id, c.user_id, c.amount, c.biz_type, c.ref_type, c.ref_id, c.remark, c.created_at
FROM `oem_sys`.`aff_commission` c
JOIN `oem_sys`.`affiliate` a ON a.id = c.oem_id
WHERE a.referrer_type = 'staff';

-- ③ 删除已迁出的员工型提成行（此后 aff_commission 仅存用户型推广提成）
DELETE c FROM `oem_sys`.`aff_commission` c
JOIN `oem_sys`.`affiliate` a ON a.id = c.oem_id
WHERE a.referrer_type = 'staff';