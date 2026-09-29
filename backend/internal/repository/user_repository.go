package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"oemrpa/internal/model"
	"oemrpa/internal/utils"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrWechatAlreadyBound 该微信号已绑定到其它账号（唯一索引冲突时返回）
	ErrWechatAlreadyBound = errors.New("wechat already bound")
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser 创建用户
func (r *UserRepository) CreateUser(user *model.User) error {
	query := `INSERT INTO ` + model.SysDB + `.user 
		(phone, username, password_hash, balance, realname_status, status, referrer_type, referrer_id, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		user.Phone,
		user.Username,
		user.PasswordHash,
		user.Balance,
		user.RealnameStatus,
		user.Status,
		user.ReferrerType,
		user.ReferrerID,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id
	return nil
}

// userColumns 平台用户常用查询列
const userColumns = `id, phone, username, password_hash, balance, realname_status, verified_name, verified_number, status, aff_code, referrer_type, referrer_id, personal_free_base, enterprise_free_base, last_login_at, created_at, updated_at, wechat_unionid, wechat_mp_openid, wechat_open_openid, wechat_nickname`

// scanUser 将查询结果扫描到 User（证件号存储加密，读取时解密）
func scanUser(row interface{ Scan(...interface{}) error }) (*model.User, error) {
	user := &model.User{}
	err := row.Scan(
		&user.ID,
		&user.Phone,
		&user.Username,
		&user.PasswordHash,
		&user.Balance,
		&user.RealnameStatus,
		&user.VerifiedName,
		&user.VerifiedNumber,
		&user.Status,
		&user.AffCode,
		&user.ReferrerType,
		&user.ReferrerID,
		&user.PersonalFreeBase,
		&user.EnterpriseFreeBase,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.WechatUnionID,
		&user.WechatMPOpenID,
		&user.WechatOpenOpenID,
		&user.WechatNickname,
	)
	if err != nil {
		return nil, err
	}
	if user.VerifiedNumber.Valid {
		user.VerifiedNumber = sql.NullString{String: utils.DecryptText(user.VerifiedNumber.String), Valid: true}
	}
	return user, nil
}

// GetUserByPhone 根据手机号查询用户
func (r *UserRepository) GetUserByPhone(phone string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user WHERE phone = ?`

	user, err := scanUser(r.db.QueryRow(query, phone))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByAccount 根据用户名/手机号查询用户（登录用）
func (r *UserRepository) GetUserByAccount(account string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user WHERE phone = ? OR username = ? ORDER BY id DESC LIMIT 1`

	user, err := scanUser(r.db.QueryRow(query, account, account))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID 根据用户ID查询用户
func (r *UserRepository) GetUserByID(id int64) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user WHERE id = ?`

	user, err := scanUser(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserCreatedAt 查询用户注册时间（用户型推广的提成归因窗口判定用）
func (r *UserRepository) GetUserCreatedAt(userID int64) (time.Time, error) {
	var t time.Time
	err := r.db.QueryRow(`SELECT created_at FROM `+model.SysDB+`.user WHERE id = ?`, userID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, ErrUserNotFound
	}
	return t, err
}

// AffSubUser 推广商视角的下级用户：仅含必要字段（白名单查询，避免泄露证件号/余额等敏感信息）
type AffSubUser struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	Phone          string    `json:"phone"`
	RealnameStatus int       `json:"realname_status"`
	Status         int       `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListAffSubUsers 分页查询归属某推广方的下级用户（仅供推广方查看自己的推广客户）
func (r *UserRepository) ListAffSubUsers(referrerType string, referrerID int64, page, pageSize int) ([]*AffSubUser, int64, error) {
	return r.ListAffSubUsersByRange(referrerType, referrerID, "", "", page, pageSize)
}

// ListAffSubUsersByRange 分页查询下级用户，可按注册时间区间过滤（销售报告按月查看）：
// startDate / endDate 为 "YYYY-MM-DD"（左闭右开，endDate 传次月 1 日；留空表示不限）。
func (r *UserRepository) ListAffSubUsersByRange(referrerType string, referrerID int64, startDate, endDate string, page, pageSize int) ([]*AffSubUser, int64, error) {
	where := ` WHERE referrer_type = ? AND referrer_id = ?`
	args := []interface{}{referrerType, referrerID}
	if startDate != "" {
		where += ` AND created_at >= ?`
		args = append(args, startDate)
	}
	if endDate != "" {
		where += ` AND created_at < ?`
		args = append(args, endDate)
	}

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.user`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`SELECT id, username, phone, realname_status, status, created_at
		FROM `+model.SysDB+`.user`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*AffSubUser, 0)
	for rows.Next() {
		u := &AffSubUser{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Phone, &u.RealnameStatus, &u.Status, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	return items, total, rows.Err()
}

// IsSelfReferral 判断下级用户与其推广商是否疑似同一人（同手机号 / 同实名主体）。
// 用于阻断「自我推广套利」：推广商用自己账号注册下级，再借下级消费刷取自己的提成。
// 仅适用于用户型推广商（referrer_type=user），referrerID 为推广商自身的 user_id；
// 员工型推广商（referrer_type=staff）的 id 与 user.id 不在同一编号空间，不在此校验范围内。
// 任一特征命中即视为疑似（宁可少计提，也不放任套利；误伤仅表现为该笔不产生提成）。
func (r *UserRepository) IsSelfReferral(userID, referrerID int64) (bool, error) {
	if userID <= 0 || referrerID <= 0 {
		return false, nil
	}
	if userID == referrerID {
		return true, nil
	}
	var same int
	err := r.db.QueryRow(`
		SELECT CASE WHEN
			u.phone = a.phone
			OR (u.verified_name IS NOT NULL AND u.verified_name <> '' AND u.verified_name = a.verified_name)
		THEN 1 ELSE 0 END
		FROM `+model.SysDB+`.user u JOIN `+model.SysDB+`.user a ON a.id = ?
		WHERE u.id = ?`, referrerID, userID).Scan(&same)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return same == 1, nil
}

// GetUserReferrer 查询用户归属的推介方（referrer_type 为空或 referrer_id<=0 表示平台直营）
func (r *UserRepository) GetUserReferrer(userID int64) (string, int64, error) {
	var referrerType string
	var referrerID int64
	err := r.db.QueryRow(`SELECT referrer_type, referrer_id FROM `+model.SysDB+`.user WHERE id = ?`, userID).Scan(&referrerType, &referrerID)
	if err == sql.ErrNoRows {
		return "", 0, ErrUserNotFound
	}
	return referrerType, referrerID, err
}

// UpdateUserReferrer 更新用户归属的推介方
func (r *UserRepository) UpdateUserReferrer(userID int64, referrerType string, referrerID int64) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.user SET referrer_type = ?, referrer_id = ?, updated_at = ? WHERE id = ?`,
		referrerType, referrerID, time.Now(), userID)
	return err
}

// ListUsersByReferrer 分页查询指定推介方的下级用户
func (r *UserRepository) ListUsersByReferrer(referrerType string, referrerID int64, page, pageSize int) ([]*model.User, int64, error) {
	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.user WHERE referrer_type = ? AND referrer_id = ?`, referrerType, referrerID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user WHERE referrer_type = ? AND referrer_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, referrerType, referrerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// GetUserByAffCode 按推广码查询用户（注册 ?ref= 绑定归属用；未命中返回 ErrUserNotFound）
func (r *UserRepository) GetUserByAffCode(code string) (*model.User, error) {
	if code == "" {
		return nil, ErrUserNotFound
	}
	u, err := scanUser(r.db.QueryRow(`SELECT `+userColumns+` FROM `+model.SysDB+`.user WHERE aff_code = ? LIMIT 1`, code))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return u, err
}

// SetUserAffCode 写入用户推广码（开通推广时生成）
func (r *UserRepository) SetUserAffCode(userID int64, code string) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.user SET aff_code = ?, updated_at = ? WHERE id = ?`, code, time.Now(), userID)
	return err
}

// AffCodeExists 判断推广码是否已被用户占用（生成时查重）
func (r *UserRepository) AffCodeExists(code string) (bool, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.user WHERE aff_code = ?`, code).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateUserPassword 更新用户密码
func (r *UserRepository) UpdateUserPassword(userID int64, passwordHash string) error {
	query := `UPDATE ` + model.SysDB + `.user SET password_hash = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, time.Now(), userID)
	return err
}

// UpdateUserRealnameInfo 更新用户实名信息（个人/企业实名共用，status 取 model.Realname* 常量；证件号存储加密）
func (r *UserRepository) UpdateUserRealnameInfo(userID int64, status int, name, number string) error {
	query := `UPDATE ` + model.SysDB + `.user 
		SET realname_status = ?, verified_name = ?, verified_number = ?, updated_at = ? 
		WHERE id = ?`
	_, err := r.db.Exec(query, status, name, utils.EncryptText(number), time.Now(), userID)
	return err
}

// UpdateRealnameFreeBase 更新实名免费次数基准偏移（管理员重置后恢复免费次数）
func (r *UserRepository) UpdateRealnameFreeBase(userID int64, personalBase, enterpriseBase int) error {
	query := `UPDATE ` + model.SysDB + `.user SET personal_free_base = ?, enterprise_free_base = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, personalBase, enterpriseBase, time.Now(), userID)
	return err
}

// GetBalanceForUpdateTx 在事务中锁定用户余额行并返回余额
func (r *UserRepository) GetBalanceForUpdateTx(tx *sql.Tx, userID int64) (float64, error) {
	var balance float64
	err := tx.QueryRow(`SELECT balance FROM `+model.SysDB+`.user WHERE id = ? FOR UPDATE`, userID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return balance, nil
}

// GetBalance 查询用户当前余额（非事务）
func (r *UserRepository) GetBalance(userID int64) (float64, error) {
	var balance float64
	err := r.db.QueryRow(`SELECT balance FROM `+model.SysDB+`.user WHERE id = ?`, userID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return balance, nil
}

// UpdateUserBalanceTx 在事务中更新用户余额
func (r *UserRepository) UpdateUserBalanceTx(tx *sql.Tx, userID int64, balance float64) error {
	query := `UPDATE ` + model.SysDB + `.user SET balance = ?, updated_at = ? WHERE id = ?`
	_, err := tx.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateLastLoginTime 更新最后登录时间
func (r *UserRepository) UpdateLastLoginTime(userID int64) error {
	query := `UPDATE ` + model.SysDB + `.user SET last_login_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), time.Now(), userID)
	return err
}

// CheckPhoneExists 检查手机号是否已存在
func (r *UserRepository) CheckPhoneExists(phone string) (bool, error) {
	query := `SELECT COUNT(*) FROM ` + model.SysDB + `.user WHERE phone = ?`
	var count int
	err := r.db.QueryRow(query, phone).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CheckUsernameExists 检查用户名是否已存在
func (r *UserRepository) CheckUsernameExists(username string) (bool, error) {
	query := `SELECT COUNT(*) FROM ` + model.SysDB + `.user WHERE username = ?`
	var count int
	err := r.db.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUsers 获取用户列表（分页 + 可选条件：关键字（手机号/用户名）、实名状态、注册时间区间）。
// realnameStatus 取 model.Realname* 常量，nil 表示不限。
func (r *UserRepository) GetAllUsers(page, pageSize int, keyword string, realnameStatus *int, startDate, endDate string) ([]*model.User, int64, error) {
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []interface{}{}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (phone LIKE ? OR username LIKE ?)"
		args = append(args, like, like)
	}
	if realnameStatus != nil {
		where += " AND realname_status = ?"
		args = append(args, *realnameStatus)
	}
	if startDate != "" {
		where += " AND created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".user WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user WHERE ` + where +
		` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]*model.User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		users = append(users, user)
	}

	return users, total, nil
}

// UpdateUserStatus 更新用户状态
func (r *UserRepository) UpdateUserStatus(userID int64, status int) error {
	query := `UPDATE ` + model.SysDB + `.user SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), userID)
	return err
}

// GetUserByWechat 按微信标识查询已绑定用户：任一标识命中即返回（unionid 命中优先）。
// 三个入参可传空串（对应端未取得该标识），空串不参与匹配；均未命中返回 ErrUserNotFound。
func (r *UserRepository) GetUserByWechat(unionID, mpOpenID, openOpenID string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM ` + model.SysDB + `.user
		WHERE (wechat_unionid IS NOT NULL AND wechat_unionid = ?)
		   OR (wechat_mp_openid IS NOT NULL AND wechat_mp_openid = ?)
		   OR (wechat_open_openid IS NOT NULL AND wechat_open_openid = ?)
		ORDER BY (wechat_unionid IS NOT NULL AND wechat_unionid = ?) DESC, id ASC
		LIMIT 1`

	user, err := scanUser(r.db.QueryRow(query, unionID, mpOpenID, openOpenID, unionID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// BindWechatMP 绑定公众号网页授权 openid（手机端一键登录）。
// unionid / nickname 为空时不覆盖已有值（同一开放平台账号下可能已由扫码端写入 unionid）。
func (r *UserRepository) BindWechatMP(userID int64, openID, unionID, nickname string) error {
	query := `UPDATE ` + model.SysDB + `.user
		SET wechat_mp_openid = ?, wechat_unionid = COALESCE(NULLIF(?, ''), wechat_unionid),
			wechat_nickname = COALESCE(NULLIF(?, ''), wechat_nickname), updated_at = ?
		WHERE id = ?`
	return wrapWechatBindErr(r.db.Exec(query, openID, unionID, nickname, time.Now(), userID))
}

// BindWechatOpen 绑定开放平台网站应用 openid（PC 扫码登录）
func (r *UserRepository) BindWechatOpen(userID int64, openID, unionID, nickname string) error {
	query := `UPDATE ` + model.SysDB + `.user
		SET wechat_open_openid = ?, wechat_unionid = COALESCE(NULLIF(?, ''), wechat_unionid),
			wechat_nickname = COALESCE(NULLIF(?, ''), wechat_nickname), updated_at = ?
		WHERE id = ?`
	return wrapWechatBindErr(r.db.Exec(query, openID, unionID, nickname, time.Now(), userID))
}

// UnbindWechatMP 解绑公众号 openid（必须写 NULL：空串会被唯一索引判为冲突）
func (r *UserRepository) UnbindWechatMP(userID int64) error {
	query := `UPDATE ` + model.SysDB + `.user SET wechat_mp_openid = NULL, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), userID)
	return err
}

// UnbindWechatOpen 解绑开放平台 openid（必须写 NULL：空串会被唯一索引判为冲突）
func (r *UserRepository) UnbindWechatOpen(userID int64) error {
	query := `UPDATE ` + model.SysDB + `.user SET wechat_open_openid = NULL, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), userID)
	return err
}

// GetWechatBinding 查询用户的微信绑定标识（未绑定的返回空串）
func (r *UserRepository) GetWechatBinding(userID int64) (unionID, mpOpenID, openOpenID, nickname string, err error) {
	var u, m, o, n sql.NullString
	err = r.db.QueryRow(`SELECT wechat_unionid, wechat_mp_openid, wechat_open_openid, wechat_nickname
		FROM `+model.SysDB+`.user WHERE id = ?`, userID).Scan(&u, &m, &o, &n)
	if err == sql.ErrNoRows {
		return "", "", "", "", ErrUserNotFound
	}
	if err != nil {
		return "", "", "", "", err
	}
	return u.String, m.String, o.String, n.String, nil
}

// wrapWechatBindErr 把唯一索引冲突（Duplicate entry）转为 ErrWechatAlreadyBound
func wrapWechatBindErr(_ sql.Result, err error) error {
	if err != nil && strings.Contains(err.Error(), "Duplicate entry") {
		return ErrWechatAlreadyBound
	}
	return err
}
