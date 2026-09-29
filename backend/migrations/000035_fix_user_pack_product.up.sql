-- 000035 修复已购资源包缺失产品标识导致无法抵扣
-- 背景：在线组合支付（/packs/:id/pay）的两条发放路径（余额全额发放、支付回调落地）此前未写入
--       user_resource_pack.product，落库为空串；而消费端预检按子产品（fv_auth/fv_self）精确匹配，
--       导致「已购资源包但余额为 0」时匹配不到资源包，回落余额校验并提示余额不足。
-- 处理：以资源包定义（resource_pack.product）为准回填用户资源包的产品快照；
--       包定义已删除/无产品的记录保持为空，由消费端的历史通用包兜底匹配承接。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（仅回填空值）。
UPDATE `oem_fv`.`user_resource_pack` u
    JOIN `oem_fv`.`resource_pack` p ON p.id = u.pack_id
SET u.product = p.product,
    u.updated_at = u.updated_at
WHERE COALESCE(u.product, '') = ''
  AND COALESCE(p.product, '') <> '';
