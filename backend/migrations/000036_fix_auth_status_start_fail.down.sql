-- 000036 回滚：将「发起失败（未扣费）」记录还原为 status=5（旧语义：5 兼容超时与发起失败）。
UPDATE `oem_fv`.`auth_record`
SET status = 5, updated_at = updated_at
WHERE status = 6
  AND COALESCE(result_code, '') = 'TIMEOUT'
  AND is_refunded = 0;
