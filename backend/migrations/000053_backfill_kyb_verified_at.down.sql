-- 000053 回滚：清除后台人工开通记录回填的 verified_at。
-- 注意：无法区分「本次回填写入的值」与「新代码开通时写入的值」（两者都等于 created_at），
--       故一并置空；回滚后列表「开通时间」回退显示 created_at，展示效果相同。
-- 幂等：仅命中 source=1 且 verified_at 等于 created_at 的行。

UPDATE `oem_sys`.`kyb`
SET verified_at = NULL
WHERE source = 1 AND verified_at IS NOT NULL AND verified_at = created_at;