package repository

import (
	"database/sql"
	"fmt"
	"time"

	"oemrpa/internal/model"
)

// SMSRepository 短信库（oem_sms）数据访问：签名 / 模板 / 发送记录
type SMSRepository struct {
	db *sql.DB
}

func NewSMSRepository(db *sql.DB) *SMSRepository {
	return &SMSRepository{db: db}
}

// SmsStatsResult 短信统计结果
type SmsStatsResult struct {
	TotalCount  int64          `json:"total_count"`  // 成功发送次数（status=0，失败不计入）
	TotalAmount float64        `json:"total_amount"` // 总消费金额：短信资源包购买费用 + 发送扣余额费用 − 已退还费用
	Daily       []SmsDailyStat `json:"daily"`        // 按日统计（升序，含近 N 天全部日期，缺失补零）
}

// SmsDailyStat 单日短信统计
type SmsDailyStat struct {
	Date   string  `json:"date"`
	Count  int64   `json:"count"`  // 当日成功发送次数
	Amount float64 `json:"amount"` // 当日消费金额（与总消费金额同口径）
}

// CountPendingSign 统计同用户相同签名内容且处于待审核/审核中的申请数
func (r *SMSRepository) CountPendingSign(userID int64, signName string) (int64, error) {
	var n int64
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_sign WHERE user_id = ? AND sign_name = ? AND status IN (0, 1)`,
		userID, signName,
	).Scan(&n)
	return n, err
}

// GetApprovedSignByName 按签名内容查询可用（本账号或公共）且已审核通过（status=2）的签名，无则返回 nil
func (r *SMSRepository) GetApprovedSignByName(userID int64, signName string) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE (user_id = ? OR is_public = 1) AND sign_name = ? AND status = 2 AND up_sign_id <> '' ORDER BY id DESC LIMIT 1`,
		userID, signName,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// CreateSign 落库签名申请（含资质材料、报备扩展字段与提交通道）
func (r *SMSRepository) CreateSign(s *model.SmsSign) error {
	query := `INSERT INTO ` + model.SmsDB + `.sms_sign
		(user_id, biz_no, sign_name, sign_type, label, credit_code_url, id_card_front, id_card_back,
		 company, legal_person, credit_code, credit_user_name, id_card, phone, sx_commits, auth_letter, screenshot,
		 channel, up_sign_id, status, result_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.Exec(query,
		s.UserID, s.BizNo, s.SignName, s.SignType, s.Label, s.CreditCodeURL, s.IDCardFront, s.IDCardBack,
		s.Company, s.LegalPerson, s.CreditCode, s.CreditUserName, s.IDCard, s.Phone, s.SxCommits, s.AuthLetter, s.Screenshot,
		s.Channel, s.UpSignID, s.Status, s.ResultMessage, time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// ListSigns 查询用户可见的签名列表（本账号 + 公共签名，分页，按提交时间倒序）
func (r *SMSRepository) ListSigns(userID int64, page, pageSize int) ([]*model.SmsSign, int64, error) {
	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_sign WHERE (user_id = ? OR is_public = 1)`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.Query(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
		COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
		COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
		COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
		status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE (user_id = ? OR is_public = 1) ORDER BY id DESC LIMIT ? OFFSET ?`,
		userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.SmsSign, 0)
	for rows.Next() {
		s := &model.SmsSign{}
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
			&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
			&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, s)
	}
	return list, total, nil
}

// GetSignByID 按主键查询签名记录（限本用户）
func (r *SMSRepository) GetSignByID(userID int64, id int64) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE user_id = ? AND id = ? LIMIT 1`,
		userID, id,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// GetSignByIDOrPublic 按主键查询可用签名（本账号或公共签名，不限 status）；供签名解析链（模板绑定/发送）使用。
// 公共签名对所有账号可见可用，故这里放宽归属校验；调用方仍须自行校验 status 与 up_sign_id。
func (r *SMSRepository) GetSignByIDOrPublic(userID int64, id int64) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE id = ? AND (user_id = ? OR is_public = 1) LIMIT 1`,
		id, userID,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// GetSignByIDForAdmin 按主键查询签名记录（后台使用，无用户隔离）
func (r *SMSRepository) GetSignByIDForAdmin(id int64) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE id = ? LIMIT 1`,
		id,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// GetSignByUpSignID 按上游签名 ID 查询签名记录（状态推送回调用，无用户隔离）
func (r *SMSRepository) GetSignByUpSignID(upSignID string) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE up_sign_id = ? LIMIT 1`,
		upSignID,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// GetLatestSignWithUpID 返回用户最近一条已报备上游（UpSignID 非空）的签名记录，无则返回 nil
func (r *SMSRepository) GetLatestSignWithUpID(userID int64) (*model.SmsSign, error) {
	s := &model.SmsSign{}
	err := r.db.QueryRow(`SELECT id, user_id, biz_no, sign_name, sign_type, label,
			COALESCE(credit_code_url, ''), COALESCE(id_card_front, ''), COALESCE(id_card_back, ''), COALESCE(company, ''), COALESCE(legal_person, ''),
			COALESCE(credit_code, ''), COALESCE(credit_user_name, ''), COALESCE(id_card, ''), COALESCE(phone, ''),
			COALESCE(sx_commits, ''), COALESCE(auth_letter, ''), COALESCE(screenshot, ''), COALESCE(channel, ''), COALESCE(up_sign_id, ''),
			status, is_public, COALESCE(result_message, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_sign WHERE user_id = ? AND up_sign_id <> '' ORDER BY id DESC LIMIT 1`,
		userID,
	).Scan(
		&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
		&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode, &s.CreditUserName,
		&s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// CreateTemplate 落库模板申请（本地存表待审核）
func (r *SMSRepository) CreateTemplate(t *model.SmsTemplate) error {
	query := `INSERT INTO ` + model.SmsDB + `.sms_template
		(user_id, template_name, template_content, template_type, sign_id, sign_name, status, template_id, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.Exec(query,
		t.UserID, t.TemplateName, t.TemplateContent, t.TemplateType, t.SignID, t.SignName,
		t.Status, t.TemplateID, t.Reason, time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

// ListTemplates 查询用户模板列表（分页，按提交时间倒序）
func (r *SMSRepository) ListTemplates(userID int64, page, pageSize int) ([]*model.SmsTemplate, int64, error) {
	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_template WHERE user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.Query(`SELECT id, user_id, template_name, template_content, template_type, COALESCE(sign_id, 0), COALESCE(sign_name, ''),
		status, COALESCE(template_id, ''), COALESCE(reason, ''), created_at, updated_at
		FROM `+model.SmsDB+`.sms_template WHERE user_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.SmsTemplate, 0)
	for rows.Next() {
		t := &model.SmsTemplate{}
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.TemplateName, &t.TemplateContent, &t.TemplateType, &t.SignID, &t.SignName,
			&t.Status, &t.TemplateID, &t.Reason, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, nil
}

// FillTemplateID 回填模板 ID（用户侧在无模板报备 API 的上游控制台创建模板后填写）
func (r *SMSRepository) FillTemplateID(userID, id int64, templateID string) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET template_id = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		templateID, time.Now(), id, userID,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("模板不存在")
	}
	return nil
}

// FillTemplateIDByID 按主键回填上游模板 ID（平台审核通过后报备上游时使用，无用户隔离）
func (r *SMSRepository) FillTemplateIDByID(id int64, templateID string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET template_id = ?, updated_at = ? WHERE id = ?`,
		templateID, time.Now(), id,
	)
	return err
}

// CreateSendRecord 落库发送记录（回填自增 ID，供账单等关联）
func (r *SMSRepository) CreateSendRecord(rec *model.SmsSendRecord) error {
	query := `INSERT INTO ` + model.SmsDB + `.sms_send_record
		(user_id, biz_no, template_id, sign_name, phone_number_set, phone_count, sms_type, message_sid, channel, request_id,
		 notify_url, status, fail_message, unit_price, amount, pay_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.Exec(query,
		rec.UserID, rec.BizNo, rec.TemplateID, rec.SignName, rec.PhoneNumberSet, rec.PhoneCount, rec.SmsType,
		rec.MessageSid, rec.Channel, rec.RequestID, rec.NotifyURL, rec.Status, rec.FailMessage, rec.UnitPrice, rec.Amount, rec.PayType, time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err == nil {
		rec.ID = id
	}
	return nil
}

// MarkSendRecordSuccess 回写发送成功结果：通道/上游任务号、按上游预扣费条数确定的计费条数与实际扣费
// （余额支付写金额、资源包扣量写扣减条数与资源包 ID）；失败原因按记录取值（成功为空，扣费异常时为对账提示）。
func (r *SMSRepository) MarkSendRecordSuccess(rec *model.SmsSendRecord) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_send_record SET status = 0, fail_message = ?, channel = ?, message_sid = ?,
			phone_count = ?, amount = ?, pay_type = ?, pack_count = ?, pack_id = ? WHERE id = ?`,
		rec.FailMessage, rec.Channel, rec.MessageSid, rec.PhoneCount, rec.Amount, rec.PayType, rec.PackCount, rec.PackID, rec.ID,
	)
	return err
}

// MarkSendRecordFailed 回写发送失败结果：保留记录并写入失败原因，未扣费故金额与资源包扣减条数置 0。
func (r *SMSRepository) MarkSendRecordFailed(rec *model.SmsSendRecord) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_send_record SET status = 1, fail_message = ?, channel = ?, message_sid = ?,
			amount = 0, pack_count = 0 WHERE id = ?`,
		rec.FailMessage, rec.Channel, rec.MessageSid, rec.ID,
	)
	return err
}

// GetSendRecordByMessageSid 按上游任务ID（message_sid）查询发送记录（回调/拉取回执用，无用户隔离）
func (r *SMSRepository) GetSendRecordByMessageSid(messageSid string) (*model.SmsSendRecord, error) {
	rec := &model.SmsSendRecord{}
	err := r.db.QueryRow(
		`SELECT id, user_id, biz_no, template_id, sign_name, phone_number_set, phone_count, sms_type,
			message_sid, channel, request_id, notify_url, status, fail_message, unit_price, amount, pay_type,
			COALESCE(pack_count, 0), COALESCE(pack_id, 0), created_at
		FROM `+model.SmsDB+`.sms_send_record WHERE message_sid = ? LIMIT 1`,
		messageSid,
	).Scan(
		&rec.ID, &rec.UserID, &rec.BizNo, &rec.TemplateID, &rec.SignName, &rec.PhoneNumberSet, &rec.PhoneCount,
		&rec.SmsType, &rec.MessageSid, &rec.Channel, &rec.RequestID, &rec.NotifyURL, &rec.Status, &rec.FailMessage, &rec.UnitPrice,
		&rec.Amount, &rec.PayType, &rec.PackCount, &rec.PackID, &rec.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// UpdateSendRecordReceipt 更新发送记录回执状态（回调/拉取回执落库）。
// success=true 时记录状态为成功并清空失败原因；否则记录失败原因（回执码 + 描述），
// 并按记录当前值回写扣费字段（回执失败已退费的记录，服务层会先把金额与资源包条数清零再落库）。
func (r *SMSRepository) UpdateSendRecordReceipt(rec *model.SmsSendRecord, success bool, failMessage string) error {
	status := 0
	if !success {
		status = 1
	}
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_send_record SET status = ?, fail_message = ?, amount = ?, pack_count = ?, pack_id = ? WHERE id = ?`,
		status, failMessage, rec.Amount, rec.PackCount, rec.PackID, rec.ID,
	)
	return err
}

// ListSendRecords 查询发送记录（分页 + 日期筛选，startDate/endDate 格式 yyyy-MM-dd）
func (r *SMSRepository) ListSendRecords(userID int64, startDate, endDate string, page, pageSize int) ([]*model.SmsSendRecord, int64, error) {
	where := `WHERE user_id = ?`
	args := []interface{}{userID}
	if startDate != "" {
		where += ` AND created_at >= ?`
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += ` AND created_at <= ?`
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_send_record `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, user_id, biz_no, template_id, sign_name, phone_number_set, phone_count, sms_type,
		message_sid, channel, request_id, status, fail_message, unit_price, amount, pay_type,
		COALESCE(pack_count, 0), COALESCE(pack_id, 0), created_at
		FROM ` + model.SmsDB + `.sms_send_record ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.SmsSendRecord, 0)
	for rows.Next() {
		rec := &model.SmsSendRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.BizNo, &rec.TemplateID, &rec.SignName, &rec.PhoneNumberSet, &rec.PhoneCount,
			&rec.SmsType, &rec.MessageSid, &rec.Channel, &rec.RequestID, &rec.Status, &rec.FailMessage, &rec.UnitPrice,
			&rec.Amount, &rec.PayType, &rec.PackCount, &rec.PackID, &rec.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, nil
}

// smsConsumeAmount 短信消费金额（统一账单口径）= 短信资源包购买费用 + 发送扣余额费用 − 已退还的短信费用。
// 资源包扣量的折算金额不计入（发送记录/账单中均记 0）；userID > 0 时仅统计该用户。
func (r *SMSRepository) smsConsumeAmount(userID int64) (float64, error) {
	query := `SELECT COALESCE(SUM(CASE WHEN bill_type = 2 THEN amount ELSE -amount END), 0)
		FROM ` + model.SysDB + `.bill
		WHERE ((bill_type = 2 AND product = ? AND spend_type IN ('balance', 'pack_purchase'))
		    OR (bill_type = 3 AND ref_type = 'sms_send_record'))`
	args := []interface{}{model.ServiceSMS}
	if userID > 0 {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	var amount float64
	if err := r.db.QueryRow(query, args...).Scan(&amount); err != nil {
		return 0, err
	}
	return amount, nil
}

// smsDailyConsumeAmounts 按天统计短信消费金额（口径同 smsConsumeAmount，按账单创建时间归日）
func (r *SMSRepository) smsDailyConsumeAmounts(userID int64, startDate, endDate string) (map[string]float64, error) {
	query := `SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS d,
		    COALESCE(SUM(CASE WHEN bill_type = 2 THEN amount ELSE -amount END), 0)
		FROM ` + model.SysDB + `.bill
		WHERE ((bill_type = 2 AND product = ? AND spend_type IN ('balance', 'pack_purchase'))
		    OR (bill_type = 3 AND ref_type = 'sms_send_record'))
		  AND DATE(created_at) >= ? AND DATE(created_at) <= ?`
	args := []interface{}{model.ServiceSMS, startDate, endDate}
	if userID > 0 {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	query += ` GROUP BY d`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var d string
		var amount float64
		if err := rows.Scan(&d, &amount); err != nil {
			return nil, err
		}
		result[d] = amount
	}
	return result, nil
}

// smsDailySuccessCounts 按天统计成功发送次数（status=0 成功，1 失败不计入）
func (r *SMSRepository) smsDailySuccessCounts(userID int64, startDate, endDate string) (map[string]int64, error) {
	query := `SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS d, COUNT(*)
		FROM ` + model.SmsDB + `.sms_send_record
		WHERE status = 0 AND DATE(created_at) >= ? AND DATE(created_at) <= ?`
	args := []interface{}{startDate, endDate}
	if userID > 0 {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	query += ` GROUP BY d`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var d string
		var count int64
		if err := rows.Scan(&d, &count); err != nil {
			return nil, err
		}
		result[d] = count
	}
	return result, nil
}

// smsDailyStats 构造近 days 天（含今天，缺失日期补零）的按日统计：成功发送次数 + 消费金额
func (r *SMSRepository) smsDailyStats(userID int64, days int) ([]SmsDailyStat, error) {
	today := time.Now()
	startDate := today.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	endDate := today.Format("2006-01-02")

	counts, err := r.smsDailySuccessCounts(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	amounts, err := r.smsDailyConsumeAmounts(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	daily := make([]SmsDailyStat, 0, days)
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
		daily = append(daily, SmsDailyStat{Date: d, Count: counts[d], Amount: amounts[d]})
	}
	return daily, nil
}

// SmsStats 短信统计：成功发送条数/总消费金额 + 近 days 天按日趋势（升序）
func (r *SMSRepository) SmsStats(userID int64, days int) (*SmsStatsResult, error) {
	res := &SmsStatsResult{}
	if err := r.db.QueryRow(
		`SELECT COALESCE(COUNT(*), 0) FROM `+model.SmsDB+`.sms_send_record WHERE user_id = ? AND status = 0`,
		userID,
	).Scan(&res.TotalCount); err != nil {
		return nil, err
	}
	amount, err := r.smsConsumeAmount(userID)
	if err != nil {
		return nil, err
	}
	res.TotalAmount = amount

	daily, err := r.smsDailyStats(userID, days)
	if err != nil {
		return nil, err
	}
	res.Daily = daily
	return res, nil
}

// AdminSmsSign 后台签名审核项（含提交用户手机号，便于识别）
type AdminSmsSign struct {
	model.SmsSign
	UserPhone string `json:"user_phone"`
}

// GetTemplateByID 按主键查询模板（限本用户；upstreamID 非空时同时按上游模板 ID 匹配）
func (r *SMSRepository) GetTemplateByID(userID int64, id int64, upstreamID string) (*model.SmsTemplate, error) {
	where := `user_id = ? AND id = ?`
	args := []interface{}{userID, id}
	if upstreamID != "" {
		where = `user_id = ? AND (id = ? OR template_id = ?)`
		args = []interface{}{userID, id, upstreamID}
	}
	t := &model.SmsTemplate{}
	err := r.db.QueryRow(
		`SELECT id, user_id, template_name, template_content, template_type, COALESCE(sign_id, 0), COALESCE(sign_name, ''), COALESCE(channel, ''),
			status, COALESCE(template_id, ''), COALESCE(reason, ''), created_at, updated_at
			FROM `+model.SmsDB+`.sms_template WHERE `+where+` LIMIT 1`,
		args...,
	).Scan(
		&t.ID, &t.UserID, &t.TemplateName, &t.TemplateContent, &t.TemplateType, &t.SignID, &t.SignName, &t.Channel,
		&t.Status, &t.TemplateID, &t.Reason, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// GetTemplateByIDForAdmin 按主键查询模板记录（后台使用，无用户隔离）
func (r *SMSRepository) GetTemplateByIDForAdmin(id int64) (*model.SmsTemplate, error) {
	t := &model.SmsTemplate{}
	err := r.db.QueryRow(
		`SELECT id, user_id, template_name, template_content, template_type, COALESCE(sign_id, 0), COALESCE(sign_name, ''), COALESCE(channel, ''),
			status, COALESCE(template_id, ''), COALESCE(reason, ''), created_at, updated_at
			FROM `+model.SmsDB+`.sms_template WHERE id = ? LIMIT 1`,
		id,
	).Scan(
		&t.ID, &t.UserID, &t.TemplateName, &t.TemplateContent, &t.TemplateType, &t.SignID, &t.SignName, &t.Channel,
		&t.Status, &t.TemplateID, &t.Reason, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// GetTemplateByUpTemplateID 按上游模板 ID 查询模板记录（状态推送回调用，无用户隔离）
func (r *SMSRepository) GetTemplateByUpTemplateID(templateID string) (*model.SmsTemplate, error) {
	t := &model.SmsTemplate{}
	err := r.db.QueryRow(
		`SELECT id, user_id, template_name, template_content, template_type, COALESCE(sign_id, 0), COALESCE(sign_name, ''), COALESCE(channel, ''),
			status, COALESCE(template_id, ''), COALESCE(reason, ''), created_at, updated_at
			FROM `+model.SmsDB+`.sms_template WHERE template_id = ? LIMIT 1`,
		templateID,
	).Scan(
		&t.ID, &t.UserID, &t.TemplateName, &t.TemplateContent, &t.TemplateType, &t.SignID, &t.SignName, &t.Channel,
		&t.Status, &t.TemplateID, &t.Reason, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// UpdateTemplateFields 更新模板名称/内容/类型/绑定签名并写入指定状态（限本用户；status 由调用方给出：
// 自有签名模板报备上游后为 0-待上游审核，公共签名模板为 3-待平台审核）
func (r *SMSRepository) UpdateTemplateFields(id, userID int64, t *model.SmsTemplate, status int) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET template_name = ?, template_content = ?, template_type = ?,
			sign_id = ?, sign_name = ?, status = ?, reason = '', updated_at = ? WHERE id = ? AND user_id = ?`,
		t.TemplateName, t.TemplateContent, t.TemplateType, t.SignID, t.SignName, status, time.Now(), id, userID,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("模板不存在")
	}
	return nil
}

// DeleteTemplate 删除模板（限本用户）
func (r *SMSRepository) DeleteTemplate(id, userID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM `+model.SmsDB+`.sms_template WHERE id = ? AND user_id = ?`, id, userID,
	)
	return err
}

// ListAllSigns 后台跨用户签名列表（可按 status 筛选，status=-1 表示全部）
func (r *SMSRepository) ListAllSigns(status int, page, pageSize int) ([]*AdminSmsSign, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	if status != -1 {
		where += ` AND s.status = ?`
		args = append(args, status)
	}

	var total int64
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_sign s `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	query := `SELECT s.id, s.user_id, s.biz_no, s.sign_name, s.sign_type, s.label,
		COALESCE(s.credit_code_url, ''), COALESCE(s.id_card_front, ''), COALESCE(s.id_card_back, ''), COALESCE(s.company, ''), COALESCE(s.legal_person, ''),
		COALESCE(s.credit_code, ''), COALESCE(s.credit_user_name, ''), COALESCE(s.id_card, ''), COALESCE(s.phone, ''),
		COALESCE(s.sx_commits, ''), COALESCE(s.auth_letter, ''), COALESCE(s.screenshot, ''), COALESCE(s.channel, ''), COALESCE(s.up_sign_id, ''),
		s.status, s.is_public, COALESCE(s.result_message, ''),
		s.created_at, s.updated_at,
		COALESCE(u.phone, '') AS user_phone
		FROM ` + model.SmsDB + `.sms_sign s
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = s.user_id ` + where +
		` ORDER BY s.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*AdminSmsSign, 0)
	for rows.Next() {
		s := &AdminSmsSign{}
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.BizNo, &s.SignName, &s.SignType, &s.Label,
			&s.CreditCodeURL, &s.IDCardFront, &s.IDCardBack, &s.Company, &s.LegalPerson, &s.CreditCode,
			&s.CreditUserName, &s.IDCard, &s.Phone, &s.SxCommits, &s.AuthLetter, &s.Screenshot, &s.Channel, &s.UpSignID, &s.Status, &s.IsPublic, &s.ResultMessage,
			&s.CreatedAt, &s.UpdatedAt, &s.UserPhone,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, s)
	}
	return list, total, nil
}

// ReviewSign 后台审核签名：status=2 通过 / 3 驳回，驳回写入原因
func (r *SMSRepository) ReviewSign(id int64, status int, reason string) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET status = ?, result_message = ?, updated_at = ? WHERE id = ? AND status IN (0, 1)`,
		status, reason, time.Now(), id,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("签名不存在或已审核")
	}
	return nil
}

// UpdateSignResultMessage 修改签名审核结果/失败原因文本（后台人工订正，不改变审核状态）
func (r *SMSRepository) UpdateSignResultMessage(id int64, message string) error {
	var cnt int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_sign WHERE id = ?`, id).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		return fmt.Errorf("签名不存在")
	}
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET result_message = ?, updated_at = ? WHERE id = ?`,
		message, time.Now(), id,
	)
	return err
}

// UpdateSignStatus 用户侧同步签名审核状态（手动查询上游审核结果后回写，按归属校验）
func (r *SMSRepository) UpdateSignStatus(id, userID int64, status int, resultMessage string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET status = ?, result_message = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		status, resultMessage, time.Now(), id, userID,
	)
	return err
}

// UpdateSignStatusByID 按主键更新签名审核状态（状态推送回调用，无用户隔离）
func (r *SMSRepository) UpdateSignStatusByID(id int64, status int, resultMessage string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET status = ?, result_message = ?, updated_at = ? WHERE id = ?`,
		status, resultMessage, time.Now(), id,
	)
	return err
}

// UpdateSignContent 更新签名报备信息（签名修改场景：新签名创建成功后，复用本记录更新资质/内容与上游ID，并重置为待审核）。
// 按用户隔离，仅更新传入的非空字段；status 固定重置为 0（待审核）等待上游再次审核。
func (r *SMSRepository) UpdateSignContent(id, userID int64, s *model.SmsSign) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET
			sign_name = ?, sign_type = ?, label = ?,
			credit_code_url = ?, id_card_front = ?, id_card_back = ?,
			company = ?, legal_person = ?, credit_code = ?, credit_user_name = ?, id_card = ?, phone = ?,
			sx_commits = ?, auth_letter = ?, screenshot = ?,
			up_sign_id = ?, status = 0, result_message = '', updated_at = ?
		WHERE id = ? AND user_id = ?`,
		s.SignName, s.SignType, s.Label,
		s.CreditCodeURL, s.IDCardFront, s.IDCardBack,
		s.Company, s.LegalPerson, s.CreditCode, s.CreditUserName, s.IDCard, s.Phone,
		s.SxCommits, s.AuthLetter, s.Screenshot,
		s.UpSignID, time.Now(), id, userID,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("签名不存在")
	}
	return nil
}

// ListAllTemplates 后台跨用户模板列表（可按 status 筛选，status=-1 表示全部）
func (r *SMSRepository) ListAllTemplates(status int, page, pageSize int) ([]*model.SmsTemplate, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	if status != -1 {
		where += ` AND status = ?`
		args = append(args, status)
	}

	var total int64
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_template `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, user_id, template_name, template_content, template_type, COALESCE(sign_id, 0), COALESCE(sign_name, ''), COALESCE(channel, ''),
		status, COALESCE(template_id, ''), COALESCE(reason, ''), created_at, updated_at
		FROM ` + model.SmsDB + `.sms_template ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.SmsTemplate, 0)
	for rows.Next() {
		t := &model.SmsTemplate{}
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.TemplateName, &t.TemplateContent, &t.TemplateType, &t.SignID, &t.SignName, &t.Channel,
			&t.Status, &t.TemplateID, &t.Reason, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, nil
}

// ReviewTemplate 后台审核模板：status=1 通过 / 2 驳回，驳回写入原因。
// 可审核「待上游审核(0)」「已通过(1)」与「待平台审核(3)」三种状态；待平台审核模板的报备上游由调用方先行完成。
func (r *SMSRepository) ReviewTemplate(id int64, status int, reason string) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET status = ?, reason = ?, updated_at = ? WHERE id = ? AND status IN (0, 1, 3)`,
		status, reason, time.Now(), id,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("模板不存在或已审核")
	}
	return nil
}

// ClaimTemplatePlatformReview 认领待平台审核模板（3 → 0，原子），返回是否认领成功。
// 平台审核通过后先认领再报备上游，避免并发重复报备。
func (r *SMSRepository) ClaimTemplatePlatformReview(id int64) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET status = 0, reason = '', updated_at = ? WHERE id = ? AND status = 3`,
		time.Now(), id,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateTemplateStatus 用户侧同步模板审核状态（手动查询上游审核结果后回写，按归属校验）
func (r *SMSRepository) UpdateTemplateStatus(id, userID int64, status int, reason string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET status = ?, reason = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		status, reason, time.Now(), id, userID,
	)
	return err
}

// UpdateTemplateStatusByID 按主键更新模板审核状态（状态推送回调用，无用户隔离）
func (r *SMSRepository) UpdateTemplateStatusByID(id int64, status int, reason string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_template SET status = ?, reason = ?, updated_at = ? WHERE id = ?`,
		status, reason, time.Now(), id,
	)
	return err
}

// GetReplyBySequenceID 按序列 ID 查询上行回复记录（去重用）
func (r *SMSRepository) GetReplyBySequenceID(sequenceID string) (*model.SmsReply, error) {
	if sequenceID == "" {
		return nil, nil
	}
	rec := &model.SmsReply{}
	err := r.db.QueryRow(
		`SELECT id, user_id, task_id, phone, sequence_id, content_down, content_up, resp_time, status, tag, created_at
			FROM `+model.SmsDB+`.sms_reply WHERE sequence_id = ? LIMIT 1`,
		sequenceID,
	).Scan(
		&rec.ID, &rec.UserID, &rec.TaskID, &rec.Phone, &rec.SequenceID, &rec.ContentDown, &rec.ContentUp,
		&rec.RespTime, &rec.Status, &rec.Tag, &rec.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// CreateReply 落库上行回复记录。已存在（重推/重复拉取）时跳过，返回 created=false。
// 去重键：推送记录用 sequence_id；拉取记录（无 sequence_id）用 task_id+phone+resp_time+content_up。
func (r *SMSRepository) CreateReply(rec *model.SmsReply) (bool, error) {
	duplicate, err := r.replyExists(rec)
	if err != nil {
		return false, err
	}
	if duplicate {
		return false, nil
	}
	_, err = r.db.Exec(
		`INSERT INTO `+model.SmsDB+`.sms_reply
			(user_id, task_id, phone, sequence_id, content_down, content_up, resp_time, status, tag, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.UserID, rec.TaskID, rec.Phone, rec.SequenceID, rec.ContentDown, rec.ContentUp,
		rec.RespTime, rec.Status, rec.Tag, time.Now(),
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// replyExists 判断回复记录是否已存在
func (r *SMSRepository) replyExists(rec *model.SmsReply) (bool, error) {
	var n int64
	var err error
	if rec.SequenceID != "" {
		err = r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_reply WHERE sequence_id = ?`, rec.SequenceID).Scan(&n)
	} else {
		err = r.db.QueryRow(
			`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_reply WHERE task_id = ? AND phone = ? AND resp_time = ? AND content_up = ?`,
			rec.TaskID, rec.Phone, rec.RespTime, rec.ContentUp,
		).Scan(&n)
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListRepliesByUser 查询用户上行回复记录（可按任务 ID 过滤，分页）
func (r *SMSRepository) ListRepliesByUser(userID int64, taskID string, page, pageSize int) ([]*model.SmsReply, int64, error) {
	where := `WHERE user_id = ?`
	args := []interface{}{userID}
	if taskID != "" {
		where += ` AND task_id = ?`
		args = append(args, taskID)
	}

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_reply `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, user_id, task_id, phone, sequence_id, content_down, content_up, resp_time, status, tag, created_at
		FROM ` + model.SmsDB + `.sms_reply ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.SmsReply, 0)
	for rows.Next() {
		rec := &model.SmsReply{}
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.TaskID, &rec.Phone, &rec.SequenceID, &rec.ContentDown, &rec.ContentUp,
			&rec.RespTime, &rec.Status, &rec.Tag, &rec.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, nil
}

// AdminSmsRecord 后台发送记录（含归属用户手机号，便于识别）
type AdminSmsRecord struct {
	model.SmsSendRecord
	UserPhone string `json:"user_phone"`
}

// ListAllSendRecords 后台跨用户发送记录列表（可按日期筛选，startDate/endDate 格式 yyyy-MM-dd）
func (r *SMSRepository) ListAllSendRecords(startDate, endDate string, page, pageSize int) ([]*AdminSmsRecord, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	if startDate != "" {
		where += ` AND s.created_at >= ?`
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += ` AND s.created_at <= ?`
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+model.SmsDB+`.sms_send_record s `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	query := `SELECT s.id, s.user_id, s.biz_no, s.template_id, s.sign_name, s.phone_number_set, s.phone_count, s.sms_type,
		s.message_sid, s.channel, s.request_id, s.status, s.fail_message, s.unit_price, s.amount, s.pay_type,
		COALESCE(s.pack_count, 0), COALESCE(s.pack_id, 0), s.created_at,
		COALESCE(u.phone, '') AS user_phone
		FROM ` + model.SmsDB + `.sms_send_record s
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = s.user_id ` + where +
		` ORDER BY s.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*AdminSmsRecord, 0)
	for rows.Next() {
		rec := &AdminSmsRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.BizNo, &rec.TemplateID, &rec.SignName, &rec.PhoneNumberSet, &rec.PhoneCount,
			&rec.SmsType, &rec.MessageSid, &rec.Channel, &rec.RequestID, &rec.Status, &rec.FailMessage, &rec.UnitPrice,
			&rec.Amount, &rec.PayType, &rec.PackCount, &rec.PackID, &rec.CreatedAt, &rec.UserPhone,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, nil
}

// SmsStatsAll 短信统计（后台全量）：成功发送条数/总消费金额 + 近 days 天按日趋势（升序）
func (r *SMSRepository) SmsStatsAll(days int) (*SmsStatsResult, error) {
	res := &SmsStatsResult{}
	if err := r.db.QueryRow(
		`SELECT COALESCE(COUNT(*), 0) FROM ` + model.SmsDB + `.sms_send_record WHERE status = 0`,
	).Scan(&res.TotalCount); err != nil {
		return nil, err
	}
	amount, err := r.smsConsumeAmount(0)
	if err != nil {
		return nil, err
	}
	res.TotalAmount = amount

	daily, err := r.smsDailyStats(0, days)
	if err != nil {
		return nil, err
	}
	res.Daily = daily
	return res, nil
}

// AdminSmsReply 后台短信上行回复（含归属用户手机号，便于识别）
type AdminSmsReply struct {
	model.SmsReply
	UserPhone string `json:"user_phone"`
}

// ListAllReplies 后台跨用户短信上行回复列表（分页）
func (r *SMSRepository) ListAllReplies(page, pageSize int) ([]*AdminSmsReply, int64, error) {
	var total int64
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM ` + model.SmsDB + `.sms_reply`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	query := `SELECT p.id, p.user_id, p.task_id, p.phone, p.sequence_id, p.content_down, p.content_up, p.resp_time, p.status, p.tag, p.created_at,
		COALESCE(u.phone, '') AS user_phone
		FROM ` + model.SmsDB + `.sms_reply p
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = p.user_id
		ORDER BY p.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*AdminSmsReply, 0)
	for rows.Next() {
		rec := &AdminSmsReply{}
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.TaskID, &rec.Phone, &rec.SequenceID, &rec.ContentDown, &rec.ContentUp,
			&rec.RespTime, &rec.Status, &rec.Tag, &rec.CreatedAt, &rec.UserPhone,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, nil
}

// ===== 公共签名 =====

// UpdateSignPublic 设置签名的公共标记（0-私有 1-公共）：仅改可见性，不动上游签名
func (r *SMSRepository) UpdateSignPublic(id int64, isPublic int) error {
	res, err := r.db.Exec(
		`UPDATE `+model.SmsDB+`.sms_sign SET is_public = ?, updated_at = ? WHERE id = ?`,
		isPublic, time.Now(), id,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("签名不存在")
	}
	return nil
}
