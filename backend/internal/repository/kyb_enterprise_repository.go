package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

// KybEnterpriseRepository 企业实名（kyb）记录仓储
type KybEnterpriseRepository struct {
	db *sql.DB
}

func NewKybEnterpriseRepository(db *sql.DB) *KybEnterpriseRepository {
	return &KybEnterpriseRepository{db: db}
}

// kybEnterpriseColumns 企业实名记录常用查询列
const kybEnterpriseColumns = `id, user_id, biz_no, 
	COALESCE(company_name, ''), COALESCE(credit_code, ''), COALESCE(legal_name, ''), COALESCE(legal_id_card, ''), 
	source, COALESCE(admin_id, 0), four_factor_status, COALESCE(four_factor_data, ''), 
	COALESCE(up_token, ''), COALESCE(up_biz_id, ''), COALESCE(up_request_id, ''), 
	status, COALESCE(result_code, ''), COALESCE(result_message, ''), COALESCE(result_data, ''), 
	verified_at, created_at, updated_at`

// scanKybEnterprise 将查询结果扫描到 KybEnterprise
func scanKybEnterprise(row interface{ Scan(...interface{}) error }) (*model.KybEnterprise, error) {
	rec := &model.KybEnterprise{}
	err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.BizNo,
		&rec.CompanyName,
		&rec.CreditCode,
		&rec.LegalName,
		&rec.LegalIDCard,
		&rec.Source,
		&rec.AdminID,
		&rec.FourFactorStatus,
		&rec.FourFactorData,
		&rec.UpToken,
		&rec.UpBizID,
		&rec.UpRequestID,
		&rec.Status,
		&rec.ResultCode,
		&rec.ResultMessage,
		&rec.ResultData,
		&rec.VerifiedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// Create 创建企业实名记录（created_at/updated_at 以记录上已设值为准，未设则取当前时间：
// 后台人工开通需与 verified_at 取同一时刻，自助流程不设值由此处兜底）
func (r *KybEnterpriseRepository) Create(rec *model.KybEnterprise) error {
	createdAt, updatedAt := rec.CreatedAt, rec.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	query := `INSERT INTO ` + model.SysDB + `.kyb 
		(user_id, biz_no, company_name, credit_code, legal_name, legal_id_card, source, admin_id, 
		four_factor_status, four_factor_data, up_token, up_biz_id, up_request_id, 
		status, result_code, result_message, result_data, verified_at, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		rec.UserID,
		rec.BizNo,
		rec.CompanyName,
		rec.CreditCode,
		rec.LegalName,
		rec.LegalIDCard,
		rec.Source,
		rec.AdminID,
		rec.FourFactorStatus,
		rec.FourFactorData,
		rec.UpToken,
		rec.UpBizID,
		rec.UpRequestID,
		rec.Status,
		rec.ResultCode,
		rec.ResultMessage,
		rec.ResultData,
		rec.VerifiedAt,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	rec.ID = id
	return nil
}

// GetByBizNo 根据业务流水号查询企业实名记录
func (r *KybEnterpriseRepository) GetByBizNo(bizNo string) (*model.KybEnterprise, error) {
	query := `SELECT ` + kybEnterpriseColumns + `
		FROM ` + model.SysDB + `.kyb WHERE biz_no = ?`
	rec, err := scanKybEnterprise(r.db.QueryRow(query, bizNo))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// GetLatestByUserID 获取用户最近一次企业实名记录
func (r *KybEnterpriseRepository) GetLatestByUserID(userID int64) (*model.KybEnterprise, error) {
	query := `SELECT ` + kybEnterpriseColumns + `
		FROM ` + model.SysDB + `.kyb WHERE user_id = ? ORDER BY created_at DESC LIMIT 1`
	rec, err := scanKybEnterprise(r.db.QueryRow(query, userID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// GetPendingByUserID 获取用户最新待法人扫脸（status=1）的企业实名记录
func (r *KybEnterpriseRepository) GetPendingByUserID(userID int64) (*model.KybEnterprise, error) {
	query := `SELECT ` + kybEnterpriseColumns + `
		FROM ` + model.SysDB + `.kyb WHERE user_id = ? AND status = 1 ORDER BY created_at DESC LIMIT 1`
	rec, err := scanKybEnterprise(r.db.QueryRow(query, userID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// UpdateUpstreamInfo 更新法人扫脸上游信息
func (r *KybEnterpriseRepository) UpdateUpstreamInfo(id int64, token, bizID, requestID string) error {
	query := `UPDATE ` + model.SysDB + `.kyb 
		SET up_token = ?, up_biz_id = ?, up_request_id = ?, status = 1, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, token, bizID, requestID, time.Now(), id)
	return err
}

// UpdateFourFactor 更新工商四要素核验结果
func (r *KybEnterpriseRepository) UpdateFourFactor(id int64, status int, data string) error {
	query := `UPDATE ` + model.SysDB + `.kyb 
		SET four_factor_status = ?, four_factor_data = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, data, time.Now(), id)
	return err
}

// UpdateResult 更新企业实名结果（成功后置状态 2 及通过时间）
func (r *KybEnterpriseRepository) UpdateResult(id int64, status int, resultCode, resultMessage, resultData string, verifiedAt *time.Time) error {
	query := `UPDATE ` + model.SysDB + `.kyb 
		SET status = ?, result_code = ?, result_message = ?, result_data = ?, verified_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, resultCode, resultMessage, resultData, verifiedAt, time.Now(), id)
	return err
}

// CountUserKybAttempts 统计企业实名自助（source=0）已发起的核验次数（成功/失败/进行中均计入，账号终身累计）。
// 免费次数按发起核验次数计：核验失败也占用免费次数。
func (r *KybEnterpriseRepository) CountUserKybAttempts(userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM ` + model.SysDB + `.kyb 
		WHERE user_id = ? AND source = 0`
	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetKybRecords 分页查询企业实名记录（管理后台）
func (r *KybEnterpriseRepository) GetKybRecords(page, pageSize int) ([]*model.KybEnterprise, int64, error) {
	countQuery := `SELECT COUNT(*) FROM ` + model.SysDB + `.kyb`
	var total int64
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT ` + kybEnterpriseColumns + `
		FROM ` + model.SysDB + `.kyb ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.KybEnterprise, 0)
	for rows.Next() {
		rec, err := scanKybEnterprise(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, nil
}
