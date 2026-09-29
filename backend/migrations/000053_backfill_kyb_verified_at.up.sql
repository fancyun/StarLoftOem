-- 000053 回填后台人工开通企业实名记录的 verified_at
-- 背景：kyb.verified_at 语义为「认证通过（开通）时间」，但后台人工开通（source=1）此前不写该列，
--       导致后台列表「开通时间」只能回退显示 created_at；代码已改为开通时写入 verified_at（与 created_at 同一时刻），
--       此处回填存量行，使该列对所有 status=2 的记录都有值。
-- 处理：source=1 且 status=2 且 verified_at 为空的行，按开通时刻（created_at）回填。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（只命中空值行，可重跑）。

UPDATE `oem_sys`.`kyb`
SET verified_at = created_at
WHERE source = 1 AND status = 2 AND verified_at IS NULL;