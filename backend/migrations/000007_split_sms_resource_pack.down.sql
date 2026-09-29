-- 000007 回滚：将短信库的资源包数据搬回 fv 库（补 product='sms'），并删除短信库独立表。
-- 注：回滚仅在确实需要还原旧结构时执行；若短信库已有新数据，回滚会丢失 product 关联，
-- 故此处按 best-effort 搬回后再删表。
INSERT INTO `oem_fv`.`resource_pack` (name, total_count, price, status, product, description, created_at, updated_at)
SELECT s.name, s.total_count, s.price, s.status, 'sms', s.description, s.created_at, s.updated_at
FROM `oem_sms`.`resource_pack` s
WHERE NOT EXISTS (
    SELECT 1 FROM `oem_fv`.`resource_pack` r
    WHERE r.product = 'sms' AND r.name = s.name AND r.total_count = s.total_count AND r.price = s.price
);

INSERT INTO `oem_fv`.`user_resource_pack` (user_id, pack_id, pack_name, total_count, remaining_count, product, status, created_at, updated_at)
SELECT s.user_id, s.pack_id, s.pack_name, s.total_count, s.remaining_count, 'sms', s.status, s.created_at, s.updated_at
FROM `oem_sms`.`user_resource_pack` s
WHERE NOT EXISTS (
    SELECT 1 FROM `oem_fv`.`user_resource_pack` u
    WHERE u.product = 'sms' AND u.user_id = s.user_id AND u.pack_name = s.pack_name AND u.total_count = s.total_count
);

DROP TABLE IF EXISTS `oem_sms`.`user_resource_pack`;
DROP TABLE IF EXISTS `oem_sms`.`resource_pack`;