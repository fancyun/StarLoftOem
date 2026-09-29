-- 000038 修正「上游结果不可取回」记录的终态语义：认证失败 → 超时结束（未完成核身，不计费已退款）
-- 背景：上游 get_result 对同一 biz_id 仅支持 3 次调用（第 4 次或超过 1 天有效期返回 403 DATA_DESTROYED）。
--       历史链路（下游轮询 / 用户回跳 / 定时任务 / 取图）无额度预算地调用该接口，把结果数据打销毁后，
--       该错误被当成「认证失败」落库（status=3、result_code=DATA_DESTROYED），
--       导致大量实际「结果不可取回、不代表核身失败」的订单被展示为认证失败，并给下游推送了失败通知。
-- 处理：① 存量 status=3 且 result_code=DATA_DESTROYED 的记录改为 status=5（超时结束：未完成核身，不计费已退款）；
--       ② 服务层后续同类记录统一落 status=5（见 internal/service/auth_service.go syncRecordResult）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（仅迁移数据销毁记录）。
UPDATE `oem_fv`.`auth_record`
SET status = 5, updated_at = updated_at
WHERE status = 3
  AND COALESCE(result_code, '') = 'DATA_DESTROYED';