package database

import (
	"fmt"
	"time"

	"oemrpa/internal/model"
)

// defaultPasswordHash 默认账号密码（kigga9aj@）的 bcrypt 哈希（cost=10），
// 用于默认管理员与默认用户，仅在对应表空表时写入。
const defaultPasswordHash = "$2a$10$rhZ7Di68AhIquIrzUclHYe6qOeBckzSNKV09tDglPCLRKNd2rMgGW"

// seedData 写入初始数据（幂等）：默认管理员与默认用户。
// 仅在对应表为空时写入，避免覆盖已有数据。需在 AutoMigrate 之后调用。
func seedData() error {
	if err := seedDefaultAdmin(); err != nil {
		return err
	}
	return seedDefaultUser()
}

// seedDefaultAdmin 在 admin_user 表无任何记录时写入默认管理员
func seedDefaultAdmin() error {
	var n int
	err := DB.QueryRow(`SELECT COUNT(*) FROM ` + model.SysDB + `.admin_user`).Scan(&n)
	if err != nil {
		return fmt.Errorf("查询管理员数量失败: %w", err)
	}
	if n > 0 {
		return nil
	}
	_, err = DB.Exec(`INSERT INTO `+model.SysDB+`.admin_user (username, password_hash, nickname, status, permissions, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)`,
		"admin", defaultPasswordHash, "管理员", model.PermissionAll, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("写入默认管理员失败: %w", err)
	}
	return nil
}

// seedDefaultUser 在 user 表无任何记录时写入默认用户
func seedDefaultUser() error {
	var n int
	err := DB.QueryRow(`SELECT COUNT(*) FROM ` + model.SysDB + `.user`).Scan(&n)
	if err != nil {
		return fmt.Errorf("查询用户数量失败: %w", err)
	}
	if n > 0 {
		return nil
	}
	_, err = DB.Exec(`INSERT INTO `+model.SysDB+`.user (phone, username, password_hash, balance, realname_status, status, created_at, updated_at)
		VALUES (?, ?, ?, 0, 0, 1, ?, ?)`,
		"13472507077", "user", defaultPasswordHash, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("写入默认用户失败: %w", err)
	}
	return nil
}
