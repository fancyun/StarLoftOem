-- 000034 回滚：恢复单一的 AFF_COMMISSION_RATE 键（人脸核验档的值），删除按产品分档的两个键
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
SELECT 'AFF_COMMISSION_RATE', s.config_value, 'aff',
       '推广提成比例（0~1，如 0.2 表示按下级成交额的 20% 计提；新推广商默认取此值）',
       NOW(), NOW()
  FROM `oem_sys`.`setting` s
 WHERE s.config_key = 'AFF_COMMISSION_RATE_FV'
ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), updated_at = NOW();

DELETE FROM `oem_sys`.`setting`
 WHERE config_key IN ('AFF_COMMISSION_RATE_FV', 'AFF_COMMISSION_RATE_SMS');
