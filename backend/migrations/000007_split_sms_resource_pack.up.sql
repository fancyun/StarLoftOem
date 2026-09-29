-- 000007 拆分短信资源包到短信库：在 oem_sms 库为短信建立独立资源包表（无 product 类型列），
-- 并将存量 fv 库中 product='sms' 的资源包定义与用户资源包搬运到短信库。
-- 说明：本迁移在 AutoMigrate 之前执行，因此需在此显式建表，避免依赖 AutoMigrate；
-- 全新库 fv 表为空，INSERT...SELECT 不产生数据，仅确保建表。
-- 禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。
CREATE TABLE IF NOT EXISTS `oem_sms`.`resource_pack` (
    `id` bigint NOT NULL AUTO_INCREMENT,
    `name` varchar(100) NOT NULL,
    `total_count` int NOT NULL,
    `price` decimal(10,2) NOT NULL,
    `status` tinyint NOT NULL DEFAULT 1,
    `description` varchar(255) DEFAULT '',
    `created_at` datetime(3) NULL,
    `updated_at` datetime(3) NULL,
    PRIMARY KEY (`id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `oem_sms`.`user_resource_pack` (
    `id` bigint NOT NULL AUTO_INCREMENT,
    `user_id` bigint NOT NULL,
    `pack_id` bigint NOT NULL,
    `pack_name` varchar(100) NOT NULL,
    `total_count` int NOT NULL,
    `remaining_count` int NOT NULL,
    `status` tinyint NOT NULL DEFAULT 1,
    `created_at` datetime(3) NULL,
    `updated_at` datetime(3) NULL,
    PRIMARY KEY (`id`),
    KEY `idx_user_status` (`user_id`,`status`),
    KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- 搬运存量短信资源包定义：只插入短信库尚不存在的（按同名+条数+价格去重幂等）
INSERT INTO `oem_sms`.`resource_pack` (name, total_count, price, status, description, created_at, updated_at)
SELECT r.name, r.total_count, r.price, r.status, r.description, r.created_at, r.updated_at
FROM `oem_fv`.`resource_pack` r
WHERE r.product = 'sms'
  AND NOT EXISTS (
      SELECT 1 FROM `oem_sms`.`resource_pack` s
      WHERE s.name = r.name AND s.total_count = r.total_count AND s.price = r.price
  );

-- 搬运存量用户短信资源包（按 user_id + pack_name + total_count 去重幂等）
INSERT INTO `oem_sms`.`user_resource_pack` (user_id, pack_id, pack_name, total_count, remaining_count, status, created_at, updated_at)
SELECT u.user_id, u.pack_id, u.pack_name, u.total_count, u.remaining_count, u.status, u.created_at, u.updated_at
FROM `oem_fv`.`user_resource_pack` u
WHERE u.product = 'sms'
  AND NOT EXISTS (
      SELECT 1 FROM `oem_sms`.`user_resource_pack` s
      WHERE s.user_id = u.user_id AND s.pack_name = u.pack_name AND s.total_count = u.total_count
  );

-- 删除 fv 库中已搬运的短信资源包（迁移完成，避免重复数据留存于 fv 库）
DELETE FROM `oem_fv`.`resource_pack` WHERE product = 'sms';
DELETE FROM `oem_fv`.`user_resource_pack` WHERE product = 'sms';