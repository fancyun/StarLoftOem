-- 回滚：删除基线迁移创建的分库（仅系统库 oem_sys 保留，由容器启动创建）。
DROP DATABASE IF EXISTS `oem_fv`;
DROP DATABASE IF EXISTS `oem_sms`;
DROP DATABASE IF EXISTS `oem_cs`;
