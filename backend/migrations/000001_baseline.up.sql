-- 基线迁移：全新数据库环境不再保留历史版本化结构变更，
-- 所有表结构由 GORM AutoMigrate 在启动时补齐（见 internal/database/db.go）。
-- 本迁移仅负责创建产品分库：系统库 oem_sys 由 MySQL 容器 MYSQL_DATABASE 自动创建，
-- 其余产品库在此补齐，避免依赖 init.sql 手动建库。
-- 各库字符集统一 utf8mb4 / utf8mb4_general_ci，与系统库一致。
-- 后续结构变更请从 000002 开始新增迁移文件。
-- 注意：禁止在迁移文件内使用 USE 切换会话库——会破坏 golang-migrate 在系统库
-- oem_sys.schema_migrations 的版本记账（表现为 Dirty database version）。
-- 跨库 DDL 一律用反引号全限定库名.表名。
CREATE DATABASE IF NOT EXISTS `oem_fv` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE DATABASE IF NOT EXISTS `oem_sms` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE DATABASE IF NOT EXISTS `oem_cs` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
