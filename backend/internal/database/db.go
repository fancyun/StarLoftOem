package database

import (
	"database/sql"
	"fmt"
	"log"
	"oemrpa/internal/config"
	"oemrpa/internal/migrate"
	"oemrpa/internal/model"
	"oemrpa/internal/utils"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var DB *sql.DB

func Init(cfg config.DatabaseConfig) error {
	// 构建DSN（连接系统库），本地 MySQL 容器无 TLS，tls=preferred 允许裸连以兼容。
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=preferred",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	DB.SetMaxIdleConns(cfg.MaxIdleConns)
	DB.SetMaxOpenConns(cfg.MaxOpenConns)
	DB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 执行数据库迁移（golang-migrate）：应用未应用的迁移版本，版本记录落系统库 schema_migrations。
	// 需在 AutoMigrate 之前执行，确保结构变更先按迁移版本收敛，再由 AutoMigrate 兜底补齐当前模型列。
	if err := migrate.Run(migrateDSN(cfg)); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// 使用 GORM AutoMigrate 自动创建/更新表结构（替代 init.sql 建表）
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to auto migrate database: %w", err)
	}

	// 写入初始数据（默认管理员 + 资源包种子，幂等）
	if err := seedData(); err != nil {
		return fmt.Errorf("failed to seed database: %w", err)
	}

	// 清理旧结构：迁移存量 API 密钥到新 api 表，并删除 user 表 api_key/api_secret 列与废弃的 user_service 表
	if err := migrateLegacyRemoval(); err != nil {
		return fmt.Errorf("failed to migrate legacy schema: %w", err)
	}

	// 存量敏感字段加密回填：历史明文（无 enc:v1: 前缀）在启动时升级为密文，幂等
	if err := backfillEncryptedFields(); err != nil {
		return fmt.Errorf("failed to backfill encrypted fields: %w", err)
	}

	// API 密钥体系迁移：一账号多把 + 逐端点权限（幂等）
	if err := backfillAPIKeys(); err != nil {
		return fmt.Errorf("failed to backfill api keys: %w", err)
	}

	log.Println("Database connected successfully with TLS support")
	return nil
}

// backfillEncryptedFields 将存量明文敏感字段（实名证件号、API Secret）加密回填。
// 加密值带 enc:v1: 前缀，已加密的行幂等跳过。
func backfillEncryptedFields() error {
	if err := backfillUserVerifiedNumber(); err != nil {
		return err
	}
	return backfillAPIKeySecret()
}

// backfillUserVerifiedNumber 逐行加密 user.verified_number 明文
func backfillUserVerifiedNumber() error {
	rows, err := DB.Query(
		`SELECT id, verified_number FROM ` + model.SysDB + `.user WHERE verified_number != '' AND verified_number NOT LIKE 'enc:v1:%'`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id     int64
		number string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.number); err != nil {
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range pending {
		enc := utils.EncryptText(r.number)
		if enc == r.number {
			continue
		}
		if _, err := DB.Exec(
			`UPDATE `+model.SysDB+`.user SET verified_number = ? WHERE id = ?`, enc, r.id,
		); err != nil {
			return err
		}
	}
	return nil
}

// backfillAPIKeySecret 逐行加密 sys.api.api_secret 明文
func backfillAPIKeySecret() error {
	rows, err := DB.Query(
		`SELECT id, api_secret FROM ` + model.SysDB + `.api WHERE api_secret != '' AND api_secret NOT LIKE 'enc:v1:%'`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id     int64
		secret string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.secret); err != nil {
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range pending {
		enc := utils.EncryptText(r.secret)
		if enc == r.secret {
			continue
		}
		if _, err := DB.Exec(
			`UPDATE `+model.SysDB+`.api SET api_secret = ? WHERE id = ?`, enc, r.id,
		); err != nil {
			return err
		}
	}
	return nil
}

// backfillAPIKeys API 密钥体系迁移（一账号多把 + 逐端点权限），幂等，需在 AutoMigrate 之后执行：
//  1. 未实名用户名下的密钥直接删除（完成实名是调用下游 API 的前提）；
//  2. 旧权限大类（fv / sms / fv,sms）展开为端点标识集合（见 model.APIEndpoints）；
//  3. 已实名但名下没有密钥的用户补一把 all 权限密钥，created_at 取实名通过时间，
//     按实名通过时间升序插入，使密钥 ID 顺序与密钥开通时间一致。
//
// 说明：新密钥的 api_secret 须用 DATA_ENCRYPT_KEY 加密后入库，无法在 SQL 迁移中生成；
// 且 api.permission 列由 AutoMigrate 扩容，故以启动回填实现（与 migrateLegacyRemoval 同源处理）。
func backfillAPIKeys() error {
	// 1. 删除未实名用户的密钥
	if _, err := DB.Exec(
		`DELETE a FROM `+model.SysDB+`.api a
		  JOIN `+model.SysDB+`.user u ON u.id = a.user_id
		 WHERE u.realname_status = ?`, model.RealnameNone,
	); err != nil {
		return err
	}

	// 2. 旧权限大类展开为端点标识集合
	classCodes := make(map[string][]string)
	endpointCodes := make([]string, 0, len(model.APIEndpoints))
	for _, ep := range model.APIEndpoints {
		classCodes[ep.Product] = append(classCodes[ep.Product], ep.Code)
		endpointCodes = append(endpointCodes, ep.Code)
	}
	for product, codes := range classCodes {
		if _, err := DB.Exec(
			`UPDATE `+model.SysDB+`.api SET permission = ?
			  WHERE REPLACE(LOWER(permission), ' ', '') = ?`,
			strings.Join(codes, ","), product,
		); err != nil {
			return err
		}
	}
	// 组合大类（fv,sms 顺序不限）展开为全部端点
	if _, err := DB.Exec(
		`UPDATE `+model.SysDB+`.api SET permission = ?
		  WHERE REPLACE(LOWER(permission), ' ', '') IN ('fv,sms', 'sms,fv')`,
		strings.Join(endpointCodes, ","),
	); err != nil {
		return err
	}

	// 3. 已实名但无密钥的用户补建 all 权限密钥
	rows, err := DB.Query(
		`SELECT u.id,
		        COALESCE((SELECT MAX(k.verified_at) FROM `+model.SysDB+`.kyc k WHERE k.user_id = u.id AND k.status = 2),
		                 (SELECT MAX(y.verified_at) FROM `+model.SysDB+`.kyb y WHERE y.user_id = u.id AND y.status = 2),
		                 u.updated_at) AS opened_at
		   FROM `+model.SysDB+`.user u
		  WHERE u.realname_status <> ?
		    AND NOT EXISTS (SELECT 1 FROM `+model.SysDB+`.api a WHERE a.user_id = u.id)
		  ORDER BY opened_at ASC, u.id ASC`, model.RealnameNone,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pendingKey struct {
		userID   int64
		openedAt time.Time
	}
	pending := make([]pendingKey, 0)
	for rows.Next() {
		var p pendingKey
		if err := rows.Scan(&p.userID, &p.openedAt); err != nil {
			return err
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range pending {
		if _, err := DB.Exec(
			`INSERT INTO `+model.SysDB+`.api (user_id, name, api_key, api_secret, permission, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'all', ?, ?)`,
			p.userID, "默认密钥", utils.GenerateRandomKey(32), utils.EncryptText(utils.GenerateRandomKey(32)), p.openedAt, p.openedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// migrateDSN 构建用于 golang-migrate 的 DSN：在业务 DSN 基础上开启 multiStatements，
// 使迁移文件内的多条语句在单连接上批量执行。
// 注意：迁移文件内禁止 USE 切换会话库（会破坏 schema_migrations 版本记账），跨库 DDL 用全限定库名.表名。
func migrateDSN(cfg config.DatabaseConfig) string {
	// golang-migrate 兼容：迁移连接走 golang-migrate mysql 驱动，该驱动不识别 tls=preferred 参数
	//（v4.16.2 已知 bug，报 "open : no such file or directory"），故此处不带 tls 参数。
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)
}

// autoMigrate 复用现有数据库连接执行 AutoMigrate，按模型自动创建表结构
func autoMigrate() error {
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: DB}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		// AutoMigrate 的 information_schema 内省查询在云数据库上耗时较长，屏蔽慢查询日志避免启动时刷屏
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		return err
	}
	// 系统库（oem_sys）：位于默认连接库，AutoMigrate 内省可直接识别已存在的表并仅补列
	migrateModels := []interface{}{
		&model.User{},
		&model.AdminUser{},
		&model.ApiKey{},
		&model.KycPersonal{},
		&model.KybEnterprise{},
		&model.PaymentOrder{},
		&model.Bill{},
		&model.NotifyRecord{},
		&model.UploadFile{},
		&model.UserLoginLog{},
		&model.AdminLoginLog{},
		&model.Setting{},
	}
	// 人脸核验库（oem_fv）：跨库表 AutoMigrate 会按当前库匹配全限定表名，
	// 导致已存在表被误判为不存在而重复 CREATE（报 1050）。这里用原生 SQL 按实际库检测，
	// 表已存在（由建库后 AutoMigrate 创建）则跳过迁移，仅迁移缺失的表
	// 各表所属库：resource_pack / user_resource_pack / auth_record 均在 FV 库
	orderTables := []struct {
		model interface{}
		table string
		db    string
	}{
		{&model.AuthRecord{}, "auth_record", model.FvDB},
		{&model.ResourcePack{}, "resource_pack", model.FvDB},
		{&model.UserResourcePack{}, "user_resource_pack", model.FvDB},
		{&model.FvProductConfig{}, "product_config", model.FvDB},
		{&model.FvPriceOverride{}, "price_override", model.FvDB},
		// 短信库（oem_sms）：签名 / 模板 / 发送记录 / 上行回复 / 短信资源包
		{&model.SmsSign{}, "sms_sign", model.SmsDB},
		{&model.SmsTemplate{}, "sms_template", model.SmsDB},
		{&model.SmsSendRecord{}, "sms_send_record", model.SmsDB},
		{&model.SmsReply{}, "sms_reply", model.SmsDB},
		{&model.SmsResourcePack{}, "resource_pack", model.SmsDB},
		{&model.SmsUserResourcePack{}, "user_resource_pack", model.SmsDB},
		{&model.SmsProductConfig{}, "product_config", model.SmsDB},
		{&model.SmsPriceOverride{}, "price_override", model.SmsDB},
		// 推广分佣（系统库 oem_sys）：账户实名两档的价格覆盖 / 提成流水 / 推广提现
		{&model.PriceOverride{}, "price_override", model.SysDB},
		{&model.UserCommission{}, "user_commission", model.SysDB},
		{&model.StaffCommission{}, "staff_commission", model.SysDB},
		{&model.PromotionWithdraw{}, "aff_withdraw", model.SysDB},
	}
	for _, t := range orderTables {
		exists, err := tableExists(t.db, t.table)
		if err != nil {
			return err
		}
		if !exists {
			migrateModels = append(migrateModels, t.model)
		}
	}

	// 日志不再落库：访问/业务/第三方调用日志均记录到本地文件（见 logstore）。
	return gormDB.AutoMigrate(migrateModels...)
}

// tableExists 判断指定库下表是否存在（跨库存在性检查基于原生查询，避免 AutoMigrate 按当前库误判）
func tableExists(schemaName, tableName string) (bool, error) {
	var n int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`,
		schemaName, tableName,
	).Scan(&n)
	return n > 0, err
}

// columnExists 判断指定库表中某列是否存在
func columnExists(schemaName, tableName, columnName string) (bool, error) {
	var n int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?`,
		schemaName, tableName, columnName,
	).Scan(&n)
	return n > 0, err
}

// migrateLegacyRemoval 移除已废弃的旧结构（仅在对应表/列仍存在时执行，具备幂等性）：
//   - 将 user 表存量 api_key/api_secret 迁入新的 api 表（permission 置为 all）
//   - 删除 user 表 api_key/api_secret 列
//   - 删除废弃的 user_service 表
func migrateLegacyRemoval() error {
	// 数据迁移：存量 API 密钥迁入 api 表
	if ex, err := columnExists(model.SysDB, "user", "api_key"); err != nil {
		return err
	} else if ex {
		if _, err := DB.Exec(`INSERT INTO ` + model.SysDB + `.api (user_id, api_key, api_secret, permission, created_at, updated_at)
			SELECT id, api_key, api_secret, 'all', NOW(), NOW()
			FROM ` + model.SysDB + `.user WHERE api_key != ''
			ON DUPLICATE KEY UPDATE api_secret = VALUES(api_secret)`); err != nil {
			return err
		}
	}

	// 删除 user 表 api_key / api_secret 列
	for _, col := range []string{"api_key", "api_secret"} {
		if ex, err := columnExists(model.SysDB, "user", col); err != nil {
			return err
		} else if ex {
			if _, err := DB.Exec(`ALTER TABLE ` + model.SysDB + `.user DROP COLUMN ` + col); err != nil {
				return err
			}
		}
	}

	// 删除废弃的 user_service 表
	if ex, err := tableExists(model.SysDB, "user_service"); err != nil {
		return err
	} else if ex {
		if _, err := DB.Exec(`DROP TABLE ` + model.SysDB + `.user_service`); err != nil {
			return err
		}
	}

	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
