-- 000045 后台权限拆分为读/写：permissions 扩列，并为存量账号补齐写权限
-- 背景：权限改为「模块读权限（无后缀）+ 模块写权限（<模块>.write）」两档，分组通配码 sys/sms/fv 覆盖其下读/写，
--       all 仍表示全部权限（超管判定只看是否持有 all，不再依赖账号 id）。
-- 处理：① permissions 由 varchar(512) 扩到 varchar(1024)（读+写全勾选约 440 字符，留足余量）；
--       ② 为存量账号把「已勾选的可写模块」补上对应 .write，保持升级前后能力不变
--          （持有 all 或分组通配码的账号无需处理：通配已覆盖写权限）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等（FIND_IN_SET 双向判定）。

-- ① 扩列
SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'admin_user' AND COLUMN_NAME = 'permissions');
SET @sql = IF(@has_col > 0,
    'ALTER TABLE `oem_sys`.`admin_user` MODIFY COLUMN `permissions` varchar(1024) NOT NULL DEFAULT '''' COMMENT ''权限码，逗号分隔；无后缀=读，.write=写，all=全部''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ② 存量补齐写权限（仅对已显式勾选的可写模块；已有 .write 或仅有通配码的不动）
UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sys.users.write')
WHERE FIND_IN_SET('sys.users', `permissions`) > 0 AND FIND_IN_SET('sys.users.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sys.finance.write')
WHERE FIND_IN_SET('sys.finance', `permissions`) > 0 AND FIND_IN_SET('sys.finance.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sys.aff.write')
WHERE FIND_IN_SET('sys.aff', `permissions`) > 0 AND FIND_IN_SET('sys.aff.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sys.admins.write')
WHERE FIND_IN_SET('sys.admins', `permissions`) > 0 AND FIND_IN_SET('sys.admins.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sys.settings.write')
WHERE FIND_IN_SET('sys.settings', `permissions`) > 0 AND FIND_IN_SET('sys.settings.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sms.signs.write')
WHERE FIND_IN_SET('sms.signs', `permissions`) > 0 AND FIND_IN_SET('sms.signs.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sms.templates.write')
WHERE FIND_IN_SET('sms.templates', `permissions`) > 0 AND FIND_IN_SET('sms.templates.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sms.packs.write')
WHERE FIND_IN_SET('sms.packs', `permissions`) > 0 AND FIND_IN_SET('sms.packs.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',sms.product_config.write')
WHERE FIND_IN_SET('sms.product_config', `permissions`) > 0 AND FIND_IN_SET('sms.product_config.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',fv.records.write')
WHERE FIND_IN_SET('fv.records', `permissions`) > 0 AND FIND_IN_SET('fv.records.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',fv.packs.write')
WHERE FIND_IN_SET('fv.packs', `permissions`) > 0 AND FIND_IN_SET('fv.packs.write', `permissions`) = 0;

UPDATE `oem_sys`.`admin_user`
SET `permissions` = CONCAT(`permissions`, ',fv.product_config.write')
WHERE FIND_IN_SET('fv.product_config', `permissions`) > 0 AND FIND_IN_SET('fv.product_config.write', `permissions`) = 0;