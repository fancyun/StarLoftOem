package migrate

import (
	"errors"
	"fmt"

	golangmigrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"oemrpa/migrations"
)

// Run 执行数据库迁移：将 backend/migrations 下未应用的版本按序应用到数据库，
// 已应用版本记录于系统库 oem_sys.schema_migrations，重复执行自动跳过。
// dsn 为普通 MySQL DSN，需带 multiStatements=true（迁移文件可能含多条语句，须在单连接批量执行）。
// 注意：迁移文件内禁止 USE 切换会话库（会破坏 golang-migrate 版本记账），跨库 DDL 用全限定库名.表名。
func Run(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("加载迁移文件失败: %w", err)
	}
	m, err := golangmigrate.NewWithSourceInstance("iofs", src, "mysql://"+dsn)
	if err != nil {
		return fmt.Errorf("初始化迁移器失败: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, golangmigrate.ErrNoChange) {
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}
	return nil
}
