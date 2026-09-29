-- 000036 拆分认证记录状态：发起失败（未扣费）从「超时已退款」中剥离
-- 背景：status=5 此前被复用为两种语义：
--       ① 发起阶段失败（上游拦截/连接超时，result_code=TIMEOUT，未产生扣费，不涉及退款）；
--       ② 真正的超时结束（核身链接过期未完成，不计费，已扣费用原路退还）。
--       前端按 status=5 一律显示「超时已退款」，导致 ① 类记录显示「已退款」而实际退款列为「否」，自相矛盾。
-- 处理：status=5 保留给「超时结束」（由新增的超时兜底任务写入，退款后 is_refunded=1）；
--       新增 status=6「发起失败（未扣费）」，并把存量 ① 类记录迁移到 6。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（仅迁移发起失败记录）。
UPDATE `oem_fv`.`auth_record`
SET status = 6, updated_at = updated_at
WHERE status = 5
  AND COALESCE(result_code, '') = 'TIMEOUT'
  AND is_refunded = 0
  AND COALESCE(cost, 0) = 0
  AND COALESCE(pack_count, 0) = 0;
