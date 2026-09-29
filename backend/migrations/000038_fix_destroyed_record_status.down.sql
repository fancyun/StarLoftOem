-- 000038 回滚：把「上游结果不可取回」记录还原为旧语义 status=3（认证失败）。
UPDATE `oem_fv`.`auth_record`
SET status = 3, updated_at = updated_at
WHERE status = 5
  AND COALESCE(result_code, '') = 'DATA_DESTROYED';