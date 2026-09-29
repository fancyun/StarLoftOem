-- 000034 推广提成比例改为「全局统一 + 按产品分档」
-- 背景：原先比例由每个推广商各自持有（affiliate.commission_rate / _fv / _sms），
--       平台无法在一处统一调整；现改为全局统一：计提一律读取系统设置
--       AFF_COMMISSION_RATE_FV / AFF_COMMISSION_RATE_SMS，推广商行内比例不再参与计提。
-- 本迁移：把旧的单一键 AFF_COMMISSION_RATE 的值迁移到两个新键，并删除旧键
--         （旧键已无代码读取，留着只会在后台「系统设置」里多出一个误导项）。
-- 说明：
--   1) 新键若已被启动预置写入（默认 0.2），以旧键的运营值覆盖，保证升级前后提成口径一致；
--      旧键不存在时本迁移不写入，由启动预置按 .env / 默认值补齐。
--   2) 推广商表的三列比例不删除，作为历史留存放行内（接口返回时会覆盖为当前生效值）。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。

-- 1.1 人脸核验档：取旧键的值作为初始值
INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
SELECT 'AFF_COMMISSION_RATE_FV', s.config_value, 'aff',
       '人脸核验提成比例（0~1，如 0.2 表示按下级成交额的 20% 计提；0 表示不提成）。全局统一，对全部推广商立即生效',
       NOW(), NOW()
  FROM `oem_sys`.`setting` s
 WHERE s.config_key = 'AFF_COMMISSION_RATE'
ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), updated_at = NOW();

-- 1.2 短信档：取旧键的值作为初始值
INSERT INTO `oem_sys`.`setting` (config_key, config_value, category, remark, created_at, updated_at)
SELECT 'AFF_COMMISSION_RATE_SMS', s.config_value, 'aff',
       '短信提成比例（0~1，如 0.2 表示按下级成交额的 20% 计提；0 表示不提成）。全局统一，对全部推广商立即生效',
       NOW(), NOW()
  FROM `oem_sys`.`setting` s
 WHERE s.config_key = 'AFF_COMMISSION_RATE'
ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), updated_at = NOW();

-- 2. 删除旧键（已无代码读取）
DELETE FROM `oem_sys`.`setting` WHERE config_key = 'AFF_COMMISSION_RATE';
