package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"

	"oemrpa/internal/audit"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SmsUpstream 短信通道上游接口：OEM 系统的唯一上游为 StarLoft 平台开放 API。
// 接口口径与业务层的状态映射保持一致（签名：1-通过 2-待审核 3-驳回 4-报备中；模板：1-通过 2-待审核 3-驳回）。
type SmsUpstream interface {
	// MarketingAvailable 营销短信通道是否可用
	MarketingAvailable() bool
	// SendSms 发送模板短信，返回平台任务号（与本平台发送记录一一对应）
	SendSms(req *upstream.SendSmsRequest) (*upstream.SendSmsResponse, error)
	// CreateSign 创建短信签名，返回平台签名记录主键
	CreateSign(req *upstream.CreateSignRequest) (*upstream.CreateSignResponse, error)
	// DeleteSign 删除短信签名
	DeleteSign(signID string) error
	// CreateFreeTemplate 报备模板，返回平台模板标识
	CreateFreeTemplate(req *upstream.CreateTemplateRequest) (string, error)
	// UpdateTemplate 修改模板内容（名称与绑定签名不随之上送）
	UpdateTemplate(templateID string, signID int64, name, content, smsType string) error
	// GetSignStatus 查询签名审核状态，返回上游状态码与驳回原因
	GetSignStatus(signID string) (int, string, error)
	// GetTemplateStatus 查询模板审核状态，返回上游状态码与驳回原因
	GetTemplateStatus(templateID, smsType string) (int, string, error)
	// PullReport 按任务号拉取发送回执
	PullReport(taskId string, pageNo, pageSize int) ([]upstream.SmsReportItem, error)
	// PullReply 按日期拉取上行回复
	PullReply(date string, pageNo, pageSize int) ([]upstream.SmsReplyItem, error)
}

// SmsChannelService 短信业务服务：
// 封装上游（StarLoft 平台开放 API）与下游用户的按子产品扣费（先扣短信资源包再扣余额），
// 并记录余额流水与发送记录（含实际使用的通道）。
// 上游未配置时返回明确的「短信通道未配置」错误，由调用方兜底提示。
type SmsChannelService struct {
	upstream         SmsUpstream // 短信产品唯一上游（StarLoft 平台）
	smsPrice         float64
	balanceService   *BalanceService
	promotionService *PromotionService
	smsRepo          *repository.SMSRepository
	apiKeyRepo       *repository.ApiKeyRepository
	notifySvc        *NotifyService
}

func NewSmsChannelService(up SmsUpstream, smsPrice float64, balanceService *BalanceService, promotionService *PromotionService, smsRepo *repository.SMSRepository, apiKeyRepo *repository.ApiKeyRepository, notifySvc *NotifyService) *SmsChannelService {
	return &SmsChannelService{
		upstream:         up,
		smsPrice:         smsPrice,
		balanceService:   balanceService,
		promotionService: promotionService,
		smsRepo:          smsRepo,
		apiKeyRepo:       apiKeyRepo,
		notifySvc:        notifySvc,
	}
}

// upstreamAvailable 上游通道是否可用（未配置时返回 false，调用方兜底提示）
func (s *SmsChannelService) upstreamAvailable() bool {
	return s.upstream != nil
}

// marketingAvailable 营销短信通道是否可用（上游未开通营销通道时不可用，与支付渠道「凭据缺失则不可用」语义一致）
func (s *SmsChannelService) marketingAvailable() bool {
	return s.upstream != nil && s.upstream.MarketingAvailable()
}

// signNamePattern 签名内容允许的字符：中文、英文、数字与中英文圆括号。
// 上游要求签名内容不含【】等符号（短信下发时由上游自动补充【】），带符号报「签名格式错误(status=39)」。
var signNamePattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9()（）]+$`)

// normalizeSignName 归一化签名内容：去除【】中括号，中文圆括号统一替换为英文圆括号。
// 短信下发时由上游自动为签名补【】，签名内容本身不应携带中括号；上游亦不接受全角圆括号。
func normalizeSignName(name string) string {
	return strings.NewReplacer("【", "", "】", "", "（", "(", "）", ")").Replace(name)
}

// validateSignName 校验签名内容格式，避免上游返回「签名格式错误(status=39)」
func validateSignName(name string) error {
	if name == "" {
		return fmt.Errorf("请输入签名内容")
	}
	if !signNamePattern.MatchString(name) {
		return fmt.Errorf("签名内容只能包含中文、英文、数字与括号，不能含【】等符号")
	}
	return nil
}

// normalizeSignLabel 归一化资质类型：0 视为默认营业执照；上游不支持 APP 报备，仅接受 1-营业执照 / 2-商标
func normalizeSignLabel(label int) (int, error) {
	switch label {
	case 0, 1:
		return 1, nil
	case 2:
		return 2, nil
	}
	return 0, fmt.Errorf("资质类型仅支持营业执照或商标")
}

// getSmsPrice 返回平台短信单价（元/条），未配置时兜底 0.05
func (s *SmsChannelService) getSmsPrice() float64 {
	if s.smsPrice > 0 {
		return s.smsPrice
	}
	return 0.05
}

// unitPrice 返回该用户适用的短信单价：推广/用户级覆盖优先，无覆盖回落平台价
func (s *SmsChannelService) unitPrice(userID int64) float64 {
	if s.promotionService != nil {
		return s.promotionService.UnitPrice(userID, model.ServiceSMS)
	}
	return s.getSmsPrice()
}

// chargeSms 短信扣费：推广 用户按「自有资源包 → 自己余额并结算给上级」计费，平台直营用户维持原语义。
// packProduct 为所扣资源包类型（sms-验证码/通知、sms_marketing-营销），由所发模板类型决定。
func (s *SmsChannelService) chargeSms(userID int64, count int64, amount float64, recordID int64, packProduct string) (int, int64, error) {
	if s.promotionService != nil {
		payType, packID, _, err := s.promotionService.ChargeUnitFee(userID, model.ServiceSMS, packProduct, count, recordID, "短信发送费用", "sms_send_record", model.CommissionBizSmsSend)
		return payType, packID, err
	}
	return s.balanceService.DeductSmsFee(userID, amount, count, recordID, "短信发送费用", "sms_send_record", packProduct)
}

// refundSms 短信退款：推广 用户按「退资源包 → 冲回上级结算」处理，平台直营用户维持原语义
func (s *SmsChannelService) refundSms(userID int64, payType int, packID int64, packCount int, amount float64, recordID int64) error {
	if s.promotionService != nil && s.promotionService.BillingReferrerID(userID) > 0 {
		if payType == model.PayTypePack && packCount > 0 {
			if packID > 0 {
				return s.balanceService.smsResourcePackRepo.RefundUserPackCount(packID, packCount)
			}
			return s.balanceService.smsResourcePackRepo.RefundUserPackCountByUser(userID, packCount)
		}
		if amount > 0 {
			return s.promotionService.RefundUnitFee(userID, model.ServiceSMS, payType, packID, amount, int64(packCount), recordID, "短信回执失败退款", "sms_send_record")
		}
		return nil
	}
	return s.balanceService.RefundSmsCharge(userID, payType, packID, packCount, amount, recordID)
}

// CreateSign 提交短信签名（下游用户，唯一上游 StarLoft 平台）：走上游创建签名（含资质材料）。
// 同签名内容待审核中禁止重复提交。提交不扣费。
func (s *SmsChannelService) CreateSign(userID int64, sign *model.SmsSign) (*upstream.CreateSignResponse, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	sign.SignName = normalizeSignName(sign.SignName)
	if err := validateSignName(sign.SignName); err != nil {
		return nil, err
	}
	if n, err := s.smsRepo.CountPendingSign(userID, sign.SignName); err != nil {
		return nil, err
	} else if n > 0 {
		return nil, fmt.Errorf("同签名待审核中，请勿重复提交")
	}

	sign.UserID = userID
	sign.BizNo = utils.GenerateRandomDigits(20)
	sign.Channel = "0" // 唯一上游 StarLoft 平台，channel 列保留固定值
	sign.Status = 0

	// 上游「本公司/他公司」是相对平台注册在上游的公司主体而言：用户报备的签名一律属于他公司
	sign.SignType = 2
	label, err := normalizeSignLabel(sign.Label)
	if err != nil {
		return nil, err
	}
	resp, err := s.upstream.CreateSign(&upstream.CreateSignRequest{
		Content:        sign.SignName,
		SignType:       sign.SignType,
		Label:          label,
		CreditCodeURL:  sign.CreditCodeURL,
		IDCardFront:    sign.IDCardFront,
		IDCardBack:     sign.IDCardBack,
		SxCommits:      sign.SxCommits,
		AuthLetter:     sign.AuthLetter,
		Screenshot:     sign.Screenshot,
		Company:        sign.Company,
		LegalPerson:    sign.LegalPerson,
		CreditCode:     sign.CreditCode,
		CreditUserName: sign.CreditUserName,
		IDCard:         sign.IDCard,
		Phone:          sign.Phone,
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != "00" {
		return nil, fmt.Errorf("上游创建签名失败: status=%s msg=%s", resp.Status, resp.Message)
	}
	sign.UpSignID = strconv.FormatInt(resp.SignId, 10)
	if err := s.smsRepo.CreateSign(sign); err != nil {
		return nil, fmt.Errorf("保存签名申请失败: %w", err)
	}
	return resp, nil
}

// UpdateSign 修改短信签名（仅已通过/驳回的签名可改；上游无修改接口，通过「删除旧签名→创建新签名」实现）。
// 流程：① 校验签名归属与状态（仅 status∈{2,3} 可改，避免审核中并发冲突）；
// ② 先删除本地记录对应的旧上游签名（同名签名已在上游存在时，直接创建会被判「签名已存在 status=25」）；
// ③ 再调上游创建新签名（新 SignId）；④ 复用本地记录更新签名内容并重置为待审核。
// 删除失败即中止（避免新旧签名在上游并存）；创建失败亦中止，此时旧签名已删除，本地记录仍指向旧 SignId，可重试。
func (s *SmsChannelService) UpdateSign(userID, id int64, sign *model.SmsSign) (*model.SmsSign, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	sign.SignName = normalizeSignName(sign.SignName)
	if err := validateSignName(sign.SignName); err != nil {
		return nil, err
	}
	old, err := s.smsRepo.GetSignByID(userID, id)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, fmt.Errorf("签名不存在")
	}
	if old.Status != 2 && old.Status != 3 {
		return nil, fmt.Errorf("仅「已通过」或「已驳回」的签名可修改")
	}
	if n, err := s.smsRepo.CountPendingSign(userID, sign.SignName); err != nil {
		return nil, err
	} else if n > 0 {
		return nil, fmt.Errorf("同签名待审核中，请勿重复提交")
	}
	sign.SignType = 2 // 上游统一按他公司主体报备
	label, err := normalizeSignLabel(sign.Label)
	if err != nil {
		return nil, err
	}

	// 先删除旧上游签名（上游同名签名只允许存在一个，否则新建报「签名已存在」）
	if old.UpSignID != "" {
		if derr := s.upstream.DeleteSign(old.UpSignID); derr != nil {
			log.Printf("修改签名删除旧上游签名失败 [sign_id=%d, up_sign_id=%s]: %v", old.ID, old.UpSignID, derr)
			return nil, fmt.Errorf("删除原签名失败，请稍后重试: %w", derr)
		}
	}

	// 再创建新签名
	resp, err := s.upstream.CreateSign(&upstream.CreateSignRequest{
		Content:        sign.SignName,
		SignType:       sign.SignType,
		Label:          label,
		CreditCodeURL:  sign.CreditCodeURL,
		IDCardFront:    sign.IDCardFront,
		IDCardBack:     sign.IDCardBack,
		SxCommits:      sign.SxCommits,
		AuthLetter:     sign.AuthLetter,
		Screenshot:     sign.Screenshot,
		Company:        sign.Company,
		LegalPerson:    sign.LegalPerson,
		CreditCode:     sign.CreditCode,
		CreditUserName: sign.CreditUserName,
		IDCard:         sign.IDCard,
		Phone:          sign.Phone,
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != "00" {
		return nil, fmt.Errorf("上游创建签名失败: status=%s msg=%s", resp.Status, resp.Message)
	}

	// 复用本地记录更新内容并重置为待审核
	sign.UpSignID = strconv.FormatInt(resp.SignId, 10)
	if err := s.smsRepo.UpdateSignContent(old.ID, userID, sign); err != nil {
		return nil, fmt.Errorf("更新签名记录失败: %w", err)
	}
	updated := *sign
	updated.ID = old.ID
	updated.BizNo = old.BizNo
	updated.Channel = "0"
	updated.Status = 0
	return &updated, nil
}

// approvedSign 解析「已审核通过且已报备上游」的签名（本账号签名或公共签名）：
// signID > 0 时按主键直取（下游传 sign_id，免去按名检索）；否则按签名内容精确匹配（归一化后按名查）。
// 未命中或未审核通过时返回 nil, nil，由调用方决定报错文案或回落策略。
func (s *SmsChannelService) approvedSign(userID, signID int64, signName string) (*model.SmsSign, error) {
	if signID > 0 {
		sign, err := s.smsRepo.GetSignByIDOrPublic(userID, signID)
		if err != nil {
			return nil, err
		}
		if sign == nil || sign.Status != 2 || sign.UpSignID == "" {
			return nil, nil
		}
		return sign, nil
	}
	if signName == "" {
		return nil, nil
	}
	return s.smsRepo.GetApprovedSignByName(userID, normalizeSignName(signName))
}

// SendSMS 发送短信（下游透传）：校验模板/签名归属与审核状态后，进入发送与计费核心流程。
// notifyURL 为下游回执主动推送地址（可选）：上游回执到达后平台将主动 POST 通知下游。
// 当前通道（上游）仅支持模板发送：必须携带 TemplateId，content 直发返回明确错误。
// signID 为签名记录主键（下游传 sign_id）：>0 时按主键直取，免去按名检索；为 0 时按 req.SignName 匹配。
// 短信类型一律取模板自身的类型（模板为通道报备实体，决定上游通道与所扣资源包），不采信请求参数。
func (s *SmsChannelService) SendSMS(userID, apiID, signID int64, req *upstream.SendSmsRequest, notifyURL string) (*upstream.SendSmsResponse, error) {
	if req.TemplateId == "" {
		return nil, fmt.Errorf("仅支持模板发送，请先创建并审核通过短信模板")
	}

	// 模板归属与状态校验：模板须属于当前账号且已审核通过，避免用他人模板发送
	localID, _ := strconv.ParseInt(req.TemplateId, 10, 64)
	tpl, err := s.smsRepo.GetTemplateByID(userID, localID, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}
	if tpl == nil {
		return nil, fmt.Errorf("模板不存在或不属于当前账号")
	}
	if tpl.Status != 1 || tpl.TemplateID == "" {
		return nil, fmt.Errorf("模板尚未审核通过，暂不可发送")
	}
	req.TemplateId = tpl.TemplateID
	req.SmsType = tpl.TemplateType

	// 签名归属与状态校验：sign_id 优先（主键直取，免去按名检索），其次 sign_name；
	// 两者都未提供时按模板绑定的签名快照记录
	if signID > 0 || req.SignName != "" {
		sign, err := s.approvedSign(userID, signID, req.SignName)
		if err != nil {
			return nil, fmt.Errorf("查询签名失败: %w", err)
		}
		if sign == nil {
			return nil, fmt.Errorf("签名不存在、不属于当前账号或尚未审核通过")
		}
		req.SignName = sign.SignName
	} else {
		req.SignName = tpl.SignName
	}

	return s.sendSMSWithResolvedSign(userID, apiID, tpl, req, notifyURL)
}

// sendSMSWithResolvedSign 模板与签名均已解析后的发送与计费核心：
// 先只校验额度（资源包条数或余额，够则放行，不预扣费），再落发送记录并发送；
// 发送失败原路退还（资源包退条数/余额退款），发送成功后按上游返回的预扣费条数校正计费（多退少补）。
// 计费条数 = 号码数 × 单条分条数；单条分条数按国内短信标准由「【签名】+ 模板正文」的长度决定（≤70 字符 1 条，
// 超出按每 67 字符 1 条递增，最多 15 条/1000 字符）。req.TemplateId 与 req.SignName 须已由调用方解析到位。
// 所扣资源包类型按模板类型解析（营销模板扣营销包，验证码/通知模板扣通用短信包），两类互不通用。
func (s *SmsChannelService) sendSMSWithResolvedSign(userID, apiID int64, tpl *model.SmsTemplate, req *upstream.SendSmsRequest, notifyURL string) (*upstream.SendSmsResponse, error) {
	packProduct := model.SmsPackProduct(tpl.TemplateType)
	if packProduct == model.ProductSMSMarketing && !s.marketingAvailable() {
		return nil, fmt.Errorf("营销短信通道未配置")
	}

	// 计费条数 = 号码数 × 单条分条数（单条按「【签名】+ 渲染后的模板正文」长度分条）
	phones := parsePhoneList(req.PhoneNumberSet)
	phoneNumbers := len(phones)
	if phoneNumbers < 1 {
		phoneNumbers = 1
	}
	var params []string
	_ = json.Unmarshal([]byte(req.TemplateParamSet), &params)
	segments := billedSmsCount("【" + req.SignName + "】" + renderTemplateContent(tpl.TemplateContent, params))
	count := phoneNumbers * segments
	unitPrice := s.unitPrice(userID)

	// 发送前只校验额度（资源包条数或余额，够则放行），不预扣费：上游报备成功后才实际扣费
	if err := s.balanceService.CheckSmsQuota(userID, count, unitPrice, packProduct); err != nil {
		return nil, err
	}

	// 先落发送记录（失败也保留该记录并写入失败原因，成功后回填计费）
	const channel = "starloft"
	rec := &model.SmsSendRecord{
		UserID:         userID,
		BizNo:          utils.GenerateRandomDigits(20),
		TemplateID:     req.TemplateId,
		SignName:       req.SignName,
		PhoneNumberSet: req.PhoneNumberSet,
		PhoneCount:     count,
		SmsType:        tpl.TemplateType,
		NotifyURL:      notifyURL,
		APIID:          apiID, // 发起该次发送的 API 密钥（下游回调按此密钥签名；控制台发送为 0）
		Channel:        channel,
		Status:         0,
		UnitPrice:      unitPrice,
	}
	if err := s.smsRepo.CreateSendRecord(rec); err != nil {
		return nil, fmt.Errorf("保存发送记录失败: %w", err)
	}
	audit.SmsSendRecord("sms_create", "api", rec, audit.KV("api_id", apiID), audit.KV("channel", channel))

	// 发送（上游）；失败不扣费，保留记录与失败原因
	if !s.upstreamAvailable() {
		return nil, s.markSendFailed(rec, "短信通道未配置")
	}
	resp, err := s.upstream.SendSms(req)
	if err != nil {
		return nil, s.markSendFailed(rec, err.Error())
	}
	if resp.Status != "" && resp.Status != "00" {
		return nil, s.markSendFailed(rec, fmt.Sprintf("上游发送短信失败: status=%s msg=%s", resp.Status, resp.Message))
	}

	// 发送成功：以上游返回的预扣费条数为准确定计费条数（本地长度仅为估算），再扣费（资源包优先、余额兜底）
	if resp.Count > 0 {
		rec.PhoneCount = resp.Count
	}
	rec.MessageSid = resp.TaskId
	amount := unitPrice * float64(rec.PhoneCount)
	payType, packID, err := s.chargeSms(userID, int64(rec.PhoneCount), amount, rec.ID, packProduct)
	if err != nil {
		// 短信已发出，扣费失败不回滚，仅记录日志与失败原因便于人工对账。
		// 计费字段一律清零：失败时未发生任何扣费，避免回执失败时按「已扣费」误退给用户。
		log.Printf("短信发送成功但扣费失败 [record_id=%d, count=%d, amount=%.4f]: %v", rec.ID, rec.PhoneCount, amount, err)
		rec.FailMessage = fmt.Sprintf("短信已发送，扣费失败: %v", err)
		rec.PayType, rec.Amount, rec.PackCount, rec.PackID = 0, 0, 0, 0
	} else if payType == model.PayTypePack {
		// 资源包扣量：金额记 0，另记本次扣减的条数与资源包（回执失败时按此退回）
		rec.PayType = payType
		rec.Amount, rec.PackCount, rec.PackID = 0, rec.PhoneCount, packID
	} else {
		rec.PayType = payType
		rec.Amount, rec.PackCount, rec.PackID = amount, 0, 0
	}
	if err := s.smsRepo.MarkSendRecordSuccess(rec); err != nil {
		log.Printf("回写发送记录失败 [record_id=%d]: %v", rec.ID, err)
	}
	audit.SmsSendRecord("sms_send", "api", rec, audit.KV("up_status", resp.Status))
	return resp, nil
}

// markSendFailed 记录发送失败：保留记录并写入失败原因，返回原错误（不扣费）
func (s *SmsChannelService) markSendFailed(rec *model.SmsSendRecord, message string) error {
	rec.Status = 1
	rec.FailMessage = message
	if err := s.smsRepo.MarkSendRecordFailed(rec); err != nil {
		log.Printf("回写发送失败记录失败 [record_id=%d]: %v", rec.ID, err)
	}
	audit.SmsSendRecord("sms_send_failed", "api", rec, audit.KV("reason", message))
	return fmt.Errorf("%s", message)
}

// parsePhoneList 解析手机号数组 JSON，失败返回空切片
func parsePhoneList(phoneJSON string) []string {
	var phones []string
	if err := json.Unmarshal([]byte(phoneJSON), &phones); err != nil {
		return nil
	}
	return phones
}

// templateVarPattern 模板变量占位符（上游格式 {%变量1%}、{%name%} 等）
var templateVarPattern = regexp.MustCompile(`\{%[^}]*%\}`)

// renderTemplateContent 把模板内容里的变量占位符按出现顺序替换为下游传入的参数值；
// 参数不足的占位符保持原样（仅供长度估算，实际渲染由上游完成）。
func renderTemplateContent(content string, params []string) string {
	if len(params) == 0 {
		return content
	}
	i := 0
	return templateVarPattern.ReplaceAllStringFunc(content, func(m string) string {
		if i >= len(params) {
			return m
		}
		v := params[i]
		i++
		return v
	})
}

// billedSmsCount 计算单条短信的计费条数（国内短信国家标准：≤70 字符计 1 条；
// 超出按每 67 字符递增 1 条，即 71-134 字 2 条、135-201 字 3 条……；最多 15 条，对应 1000 字符上限）。
func billedSmsCount(text string) int {
	n := utf8.RuneCountInString(text)
	if n <= 70 {
		return 1
	}
	segments := (n + 66) / 67
	if segments > 15 {
		segments = 15
	}
	return segments
}

// ListSigns 查询用户签名申请列表（分页）
func (s *SmsChannelService) ListSigns(userID int64, page, pageSize int) ([]*model.SmsSign, int64, error) {
	return s.smsRepo.ListSigns(userID, page, pageSize)
}

// GetSign 查询单条签名（限本用户，供修改页回填表单）
func (s *SmsChannelService) GetSign(userID, id int64) (*model.SmsSign, error) {
	return s.smsRepo.GetSignByID(userID, id)
}

// GetTemplate 查询单条模板（限本用户，供修改页回填表单）
func (s *SmsChannelService) GetTemplate(userID, id int64) (*model.SmsTemplate, error) {
	return s.smsRepo.GetTemplateByID(userID, id, "")
}

// signForTemplate 取模板绑定的签名：须为可用签名（本账号或公共）、已审核通过且已报备上游
// （上游创建/编辑模板要求 SignId 为审核通过的签名）
func (s *SmsChannelService) signForTemplate(userID, signID int64) (*model.SmsSign, error) {
	if signID <= 0 {
		return nil, fmt.Errorf("请选择已审核通过的短信签名")
	}
	sign, err := s.smsRepo.GetSignByIDOrPublic(userID, signID)
	if err != nil {
		return nil, err
	}
	if sign == nil {
		return nil, fmt.Errorf("签名不存在或不可用")
	}
	if sign.Status != 2 || sign.UpSignID == "" {
		return nil, fmt.Errorf("签名尚未审核通过，请选择已审核通过的签名")
	}
	return sign, nil
}

// signNeedsPlatformReview 绑定公共签名（is_public=1，平台提供的签名）的模板须先经平台审核，
// 通过后才报备上游；自有签名模板提交/修改即报备上游。
func signNeedsPlatformReview(sign *model.SmsSign) bool {
	return sign != nil && sign.IsPublic == 1
}

// reportTemplateUpstream 向上游报备模板：尚无上游模板 ID 时新建，已有则按当前内容编辑，返回上游模板 ID
func (s *SmsChannelService) reportTemplateUpstream(t *model.SmsTemplate, upSignID string) (string, error) {
	signId, _ := strconv.ParseInt(upSignID, 10, 64)
	if t.TemplateID == "" {
		return s.upstream.CreateFreeTemplate(&upstream.CreateTemplateRequest{
			SignId:       signId,
			TemplateName: t.TemplateName,
			Content:      t.TemplateContent,
			SmsType:      t.TemplateType,
		})
	}
	if err := s.upstream.UpdateTemplate(t.TemplateID, signId, t.TemplateName, t.TemplateContent, t.TemplateType); err != nil {
		return "", err
	}
	return t.TemplateID, nil
}

// CreateTemplate 提交模板申请（控制台用户）：绑定所选签名后报备上游。
// 自有签名模板提交即报备上游（待上游审核）；公共签名模板先落库为待平台审核，平台审核通过后才报备上游。
func (s *SmsChannelService) CreateTemplate(userID int64, t *model.SmsTemplate) error {
	if !s.upstreamAvailable() {
		return fmt.Errorf("短信通道未配置")
	}
	smsType, err := normalizeSmsType(t.TemplateType)
	if err != nil {
		return err
	}
	if smsType == model.SmsTemplateTypeMarketing && !s.marketingAvailable() {
		return fmt.Errorf("营销短信通道未配置")
	}
	sign, err := s.signForTemplate(userID, t.SignID)
	if err != nil {
		return err
	}

	t.UserID = userID
	t.Channel = "0" // 唯一上游 StarLoft 平台，channel 列保留固定值
	t.TemplateType = smsType
	t.Status = model.SmsTemplateStatusPending
	t.SignID = sign.ID
	t.SignName = sign.SignName

	if signNeedsPlatformReview(sign) {
		t.Status = model.SmsTemplateStatusPlatformReview
		return s.smsRepo.CreateTemplate(t)
	}

	tplID, err := s.reportTemplateUpstream(t, sign.UpSignID)
	if err != nil {
		return fmt.Errorf("上游模板报备失败: %w", err)
	}
	t.TemplateID = tplID
	return s.smsRepo.CreateTemplate(t)
}

// UpdateTemplate 修改模板（控制台用户）：仅「已通过/已驳回」可改。
// 自有签名模板按「名称 + 内容 + 绑定签名」整体提交上游后重置为待上游审核；
// 改绑公共签名时先重置为待平台审核（不提交上游），平台审核通过后才按新内容报备上游。
func (s *SmsChannelService) UpdateTemplate(userID, id int64, t *model.SmsTemplate) (*model.SmsTemplate, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	smsType, err := normalizeSmsType(t.TemplateType)
	if err != nil {
		return nil, err
	}
	if smsType == model.SmsTemplateTypeMarketing && !s.marketingAvailable() {
		return nil, fmt.Errorf("营销短信通道未配置")
	}
	old, err := s.smsRepo.GetTemplateByID(userID, id, "")
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, fmt.Errorf("模板不存在")
	}
	if old.Status != 1 && old.Status != 2 {
		return nil, fmt.Errorf("仅「已通过」或「已驳回」的模板可修改")
	}
	if old.TemplateID == "" {
		return nil, fmt.Errorf("模板尚未报备上游，无法修改")
	}
	sign, err := s.signForTemplate(userID, t.SignID)
	if err != nil {
		return nil, err
	}

	updated := *t
	updated.ID = old.ID
	updated.UserID = userID
	updated.SignID = sign.ID
	updated.SignName = sign.SignName
	updated.TemplateType = smsType
	updated.Channel = "0"
	updated.TemplateID = old.TemplateID
	updated.Reason = ""
	updated.Status = model.SmsTemplateStatusPending

	// 改绑公共签名：先平台审核（不提交上游）；自有签名：按新内容整体提交上游后等上游审核
	if signNeedsPlatformReview(sign) {
		updated.Status = model.SmsTemplateStatusPlatformReview
	} else if _, err := s.reportTemplateUpstream(&updated, sign.UpSignID); err != nil {
		return nil, fmt.Errorf("上游模板编辑失败: %w", err)
	}

	if err := s.smsRepo.UpdateTemplateFields(old.ID, userID, &updated, updated.Status); err != nil {
		return nil, fmt.Errorf("更新模板记录失败: %w", err)
	}
	updated.CreatedAt = old.CreatedAt
	return &updated, nil
}

// templateSignForAPI 解析下游 API 报备模板所用的签名：sign_id 优先（主键直取，免去按名检索），
// 其次 sign_name 按名匹配；两者都未指定时回落该用户最近一条已审核通过且已报备上游的签名。
func (s *SmsChannelService) templateSignForAPI(userID, signID int64, signName string) (*model.SmsSign, error) {
	if signID > 0 || signName != "" {
		sign, err := s.approvedSign(userID, signID, signName)
		if err != nil {
			return nil, err
		}
		if sign == nil {
			return nil, fmt.Errorf("签名不存在、不属于当前账号或尚未审核通过")
		}
		return sign, nil
	}
	sign, err := s.smsRepo.GetLatestSignWithUpID(userID)
	if err != nil {
		return nil, err
	}
	if sign == nil || sign.Status != 2 {
		return nil, fmt.Errorf("请先在控制台提交并通过短信签名，再创建模板")
	}
	return sign, nil
}

// CreateTemplateForAPI 下游 API 创建模板：绑定签名后上报上游报备（返回模板记录）。
// 签名以 sign_id 优先（主键直取，免去按名检索），未传时按 sign_name 匹配；签名内容以实际命中的签名记录为准；
// 命中的签名可为账号自有签名或公共签名。
func (s *SmsChannelService) CreateTemplateForAPI(userID int64, name, content, smsType string, signID int64, signName string) (*model.SmsTemplate, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	normalized, err := normalizeSmsType(smsType)
	if err != nil {
		return nil, err
	}
	if normalized == model.SmsTemplateTypeMarketing && !s.marketingAvailable() {
		return nil, fmt.Errorf("营销短信通道未配置")
	}
	t := &model.SmsTemplate{
		UserID:          userID,
		TemplateName:    name,
		TemplateContent: content,
		TemplateType:    normalized,
		Channel:         "0", // 唯一上游 StarLoft 平台
		Status:          model.SmsTemplateStatusPending,
	}

	sign, err := s.templateSignForAPI(userID, signID, signName)
	if err != nil {
		return nil, err
	}
	t.SignID = sign.ID
	t.SignName = sign.SignName
	// 公共签名模板：先落库待平台审核，平台审核通过后才报备上游
	if signNeedsPlatformReview(sign) {
		t.Status = model.SmsTemplateStatusPlatformReview
		return t, s.smsRepo.CreateTemplate(t)
	}
	tplID, err := s.reportTemplateUpstream(t, sign.UpSignID)
	if err != nil {
		return nil, fmt.Errorf("上游模板报备失败: %w", err)
	}
	t.TemplateID = tplID
	if err := s.smsRepo.CreateTemplate(t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTemplateForAPI 查询下游模板（按主键或上游模板 ID）
func (s *SmsChannelService) GetTemplateForAPI(userID int64, id int64, upstreamID string) (*model.SmsTemplate, error) {
	return s.smsRepo.GetTemplateByID(userID, id, upstreamID)
}

// UpdateTemplateForAPI 修改下游模板内容：自有签名模板走上游编辑接口（上游模板 ID 不变）并重置为待上游审核；
// 绑定公共签名的模板改内容同样先平台审核（不提交上游），通过后才按新内容提交上游。
// 原绑定签名失效（被驳回/删除）时回落该用户最近一条已审核通过的签名。
func (s *SmsChannelService) UpdateTemplateForAPI(userID int64, id int64, content string) (*model.SmsTemplate, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	old, err := s.smsRepo.GetTemplateByID(userID, id, "")
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, fmt.Errorf("模板不存在")
	}
	if old.TemplateID == "" {
		return nil, fmt.Errorf("模板尚未报备上游，无法修改")
	}
	sign, err := s.signForTemplate(userID, old.SignID)
	if err != nil {
		if sign, err = s.templateSignForAPI(userID, 0, ""); err != nil {
			return nil, fmt.Errorf("请先在控制台提交并通过短信签名，再修改模板")
		}
	}
	old.TemplateContent = content
	old.Channel = "0"
	old.SignID = sign.ID
	old.SignName = sign.SignName
	old.Reason = ""
	old.Status = model.SmsTemplateStatusPending
	if signNeedsPlatformReview(sign) {
		old.Status = model.SmsTemplateStatusPlatformReview
	} else if _, err := s.reportTemplateUpstream(old, sign.UpSignID); err != nil {
		return nil, fmt.Errorf("上游模板编辑失败: %w", err)
	}
	if err := s.smsRepo.UpdateTemplateFields(id, userID, old, old.Status); err != nil {
		return nil, err
	}
	return old, nil
}

// DeleteTemplateForAPI 删除下游模板
func (s *SmsChannelService) DeleteTemplateForAPI(userID int64, id int64) error {
	return s.smsRepo.DeleteTemplate(id, userID)
}

// ListSignsForAdmin 后台签名审核列表（status=-1 全部）
func (s *SmsChannelService) ListSignsForAdmin(status, page, pageSize int) ([]*repository.AdminSmsSign, int64, error) {
	return s.smsRepo.ListAllSigns(status, page, pageSize)
}

// ReviewSign 后台人工审核签名：status=2 通过 / 3 驳回，驳回写入原因
func (s *SmsChannelService) ReviewSign(id int64, status int, reason string) error {
	if err := s.smsRepo.ReviewSign(id, status, reason); err != nil {
		return err
	}
	audit.Log("sign_review", audit.KV("source", "admin"), audit.KV("sign_id", id),
		audit.KV("status", status), audit.KV("reason", reason))
	return nil
}

// UpdateSignResultMessage 后台修改签名审核结果/失败原因文本（不改变审核状态）
func (s *SmsChannelService) UpdateSignResultMessage(id int64, message string) error {
	if err := s.smsRepo.UpdateSignResultMessage(id, message); err != nil {
		return err
	}
	audit.Log("sign_review_message", audit.KV("source", "admin"), audit.KV("sign_id", id), audit.KV("reason", message))
	return nil
}

// ListTemplatesForAdmin 后台模板审核列表（status=-1 全部）
func (s *SmsChannelService) ListTemplatesForAdmin(status, page, pageSize int) ([]*model.SmsTemplate, int64, error) {
	return s.smsRepo.ListAllTemplates(status, page, pageSize)
}

// ReviewTemplate 后台人工审核模板：status=1 通过 / 2 驳回，驳回写入原因。
// 「待平台审核（3）」的模板由平台先行把关：通过时先报备上游（报备失败回滚为待平台审核，可修正后重试），
// 之后与自有签名模板一样等待上游审核结果；其余状态按现状直接置位。
func (s *SmsChannelService) ReviewTemplate(id int64, status int, reason string) error {
	t, err := s.smsRepo.GetTemplateByIDForAdmin(id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("模板不存在")
	}

	if t.Status == model.SmsTemplateStatusPlatformReview && status == 1 {
		if err := s.approveTemplatePlatformReview(t); err != nil {
			return err
		}
	} else if err := s.smsRepo.ReviewTemplate(id, status, reason); err != nil {
		return err
	}

	audit.Log("template_review", audit.KV("source", "admin"), audit.KV("template_id", id),
		audit.KV("status", status), audit.KV("reason", reason))
	return nil
}

// approveTemplatePlatformReview 平台审核通过：先原子认领（3→0，防并发重复报备）再报备上游；
// 报备失败回滚为待平台审核并返回错误（管理员可修正后重试）。
func (s *SmsChannelService) approveTemplatePlatformReview(t *model.SmsTemplate) error {
	if !s.upstreamAvailable() {
		return fmt.Errorf("短信通道未配置")
	}
	sign, err := s.smsRepo.GetSignByIDForAdmin(t.SignID)
	if err != nil {
		return err
	}
	if sign == nil || sign.Status != 2 || sign.UpSignID == "" {
		return fmt.Errorf("关联签名不可用（未审核通过或未报备上游），请先处理签名")
	}
	claimed, err := s.smsRepo.ClaimTemplatePlatformReview(t.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return fmt.Errorf("模板状态已变更，请刷新后重试")
	}

	tplID, err := s.reportTemplateUpstream(t, sign.UpSignID)
	if err != nil {
		// 报备失败：回滚为待平台审核，避免模板卡在「已报备」状态
		if rerr := s.smsRepo.UpdateTemplateStatusByID(t.ID, model.SmsTemplateStatusPlatformReview, ""); rerr != nil {
			log.Printf("回滚模板待平台审核状态失败 [template_id=%d]: %v", t.ID, rerr)
		}
		return fmt.Errorf("报备上游失败: %w", err)
	}
	if tplID != t.TemplateID {
		if err := s.smsRepo.FillTemplateIDByID(t.ID, tplID); err != nil {
			return fmt.Errorf("回填上游模板 ID 失败: %w", err)
		}
	}
	return nil
}

// ===== 公共签名（验证码服务，后台管理）=====

// SetSignPublic 设置签名的公共标记（0-私有 1-公共）：仅改可见性，不动上游签名。
// 公共签名对所有账号可见、可被任意账号绑定模板使用；仅「已审核通过且已报备上游」的签名可设为公共。
func (s *SmsChannelService) SetSignPublic(id int64, isPublic int) error {
	if isPublic != 0 && isPublic != 1 {
		return fmt.Errorf("公共标记无效")
	}
	sign, err := s.smsRepo.GetSignByIDForAdmin(id)
	if err != nil {
		return err
	}
	if sign == nil {
		return fmt.Errorf("签名不存在")
	}
	if isPublic == 1 && (sign.Status != 2 || sign.UpSignID == "") {
		return fmt.Errorf("仅「已审核通过」的签名可设为公共")
	}
	if err := s.smsRepo.UpdateSignPublic(id, isPublic); err != nil {
		return err
	}
	audit.Log("sign_public", audit.KV("source", "admin"), audit.KV("sign_id", id),
		audit.KV("user_id", sign.UserID), audit.KV("is_public", isPublic), audit.KV("sign_name", sign.SignName))
	return nil
}

// normalizeSmsType 校验并归一短信类型（验证码/通知/营销；空值按通知处理）
func normalizeSmsType(t string) (string, error) {
	if t == "" {
		return model.SmsTemplateTypeNotify, nil
	}
	switch t {
	case model.SmsTemplateTypeVerify, model.SmsTemplateTypeNotify, model.SmsTemplateTypeMarketing:
		return t, nil
	}
	return "", fmt.Errorf("暂不支持该短信类型（当前仅支持验证码/通知/营销短信）")
}

// QuerySignStatus 手动查询签名审核状态（用户侧）：调上游（上游）核对并回写本地
func (s *SmsChannelService) QuerySignStatus(userID, id int64) (*model.SmsSign, error) {
	sign, err := s.querySignStatus(userID, id)
	auditSign(sign, "console", "sign_query")
	return sign, err
}

// QuerySignStatusForAdmin 后台手动查询签名审核状态：与用户侧同一套比对逻辑，不校验签名归属
func (s *SmsChannelService) QuerySignStatusForAdmin(id int64) (*model.SmsSign, error) {
	sign, err := s.querySignStatus(0, id)
	auditSign(sign, "admin", "sign_query")
	return sign, err
}

// auditSign 签名状态变更审计（含审核状态与结果文本，便于日后按 sign_id 复盘回写）
func auditSign(sign *model.SmsSign, source, action string) {
	if sign == nil {
		return
	}
	audit.Log(action,
		audit.KV("source", source),
		audit.KV("sign_id", sign.ID),
		audit.KV("user_id", sign.UserID),
		audit.KV("up_sign_id", sign.UpSignID),
		audit.KV("status", sign.Status),
		audit.KV("sign_name", sign.SignName),
		audit.KV("result_message", sign.ResultMessage))
}

// auditTemplate 模板状态变更审计（便于日后按 template_id 复盘回写）
func auditTemplate(t *model.SmsTemplate, source, action string) {
	if t == nil {
		return
	}
	audit.Log(action,
		audit.KV("source", source),
		audit.KV("template_id", t.ID),
		audit.KV("user_id", t.UserID),
		audit.KV("up_template_id", t.TemplateID),
		audit.KV("status", t.Status),
		audit.KV("reason", t.Reason))
}

// querySignStatus 调上游核对签名审核状态并回写本地；userID<=0 表示后台调用，不校验归属。
// 以上游状态为准（含本地已置为通过但上游仍在审核的情况）：
// 1-通过→2（清空结果文本）、3-驳回→3（写入驳回原因）、2-待审核→1「审核中」、4-报备中→1「报备中」。
func (s *SmsChannelService) querySignStatus(userID, id int64) (*model.SmsSign, error) {
	var (
		sign *model.SmsSign
		err  error
	)
	if userID > 0 {
		sign, err = s.smsRepo.GetSignByID(userID, id)
	} else {
		sign, err = s.smsRepo.GetSignByIDForAdmin(id)
	}
	if err != nil {
		return nil, err
	}
	if sign == nil {
		return nil, fmt.Errorf("签名不存在")
	}
	if sign.UpSignID == "" {
		return sign, nil
	}
	upStatus, refuse, err := s.upstream.GetSignStatus(sign.UpSignID)
	if err != nil {
		return nil, err
	}
	newStatus := sign.Status
	resultMsg := sign.ResultMessage
	switch upStatus {
	case 1:
		newStatus, resultMsg = 2, ""
	case 3:
		newStatus, resultMsg = 3, refuse
	case 2:
		newStatus, resultMsg = 1, "审核中"
	case 4:
		newStatus, resultMsg = 1, "报备中"
	}
	if newStatus != sign.Status || resultMsg != sign.ResultMessage {
		if userID > 0 {
			err = s.smsRepo.UpdateSignStatus(sign.ID, userID, newStatus, resultMsg)
		} else {
			err = s.smsRepo.UpdateSignStatusByID(sign.ID, newStatus, resultMsg)
		}
		if err != nil {
			return nil, err
		}
		sign.Status = newStatus
		sign.ResultMessage = resultMsg
	}
	return sign, nil
}

// QueryTemplateStatus 手动查询模板审核状态（用户侧）：调上游（上游）核对并回写本地
func (s *SmsChannelService) QueryTemplateStatus(userID, id int64) (*model.SmsTemplate, error) {
	t, err := s.queryTemplateStatus(userID, id)
	auditTemplate(t, "console", "template_query")
	return t, err
}

// QueryTemplateStatusForAdmin 后台手动查询模板审核状态：与用户侧同一套比对逻辑，不校验模板归属
func (s *SmsChannelService) QueryTemplateStatusForAdmin(id int64) (*model.SmsTemplate, error) {
	t, err := s.queryTemplateStatus(0, id)
	auditTemplate(t, "admin", "template_query")
	return t, err
}

// queryTemplateStatus 调上游核对模板审核状态并回写本地；userID<=0 表示后台调用，不校验归属。
// 以上游状态为准（含本地已置为通过但上游仍在审核的情况）：
// 1-通过→1（清空说明）、3-驳回→2（写入驳回原因）、2-待审核→0「审核中」。
// 待平台审核（3）不查询上游：该状态下最新内容尚未报备上游，上游结果属于旧内容，不能据此解锁。
func (s *SmsChannelService) queryTemplateStatus(userID, id int64) (*model.SmsTemplate, error) {
	var (
		t   *model.SmsTemplate
		err error
	)
	if userID > 0 {
		t, err = s.smsRepo.GetTemplateByID(userID, id, "")
	} else {
		t, err = s.smsRepo.GetTemplateByIDForAdmin(id)
	}
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("模板不存在")
	}
	if t.TemplateID == "" || t.Status == model.SmsTemplateStatusPlatformReview {
		return t, nil
	}
	upStatus, refuse, err := s.upstream.GetTemplateStatus(t.TemplateID, t.TemplateType)
	if err != nil {
		return nil, err
	}
	newStatus := t.Status
	reason := t.Reason
	switch upStatus {
	case 1:
		newStatus, reason = 1, ""
	case 3:
		newStatus, reason = 2, refuse
	case 2:
		newStatus, reason = 0, "审核中"
	}
	if newStatus != t.Status || reason != t.Reason {
		if userID > 0 {
			err = s.smsRepo.UpdateTemplateStatus(t.ID, userID, newStatus, reason)
		} else {
			err = s.smsRepo.UpdateTemplateStatusByID(t.ID, newStatus, reason)
		}
		if err != nil {
			return nil, err
		}
		t.Status = newStatus
		t.Reason = reason
	}
	return t, nil
}

// ListTemplates 查询用户模板列表（分页）
func (s *SmsChannelService) ListTemplates(userID int64, page, pageSize int) ([]*model.SmsTemplate, int64, error) {
	return s.smsRepo.ListTemplates(userID, page, pageSize)
}

// HandleSignStatusPush 处理上游签名状态推送（回调，无签名验签）。
// 按上游 SignId（up_sign_id）关联本地签名；status=1 通过→2、3 驳回→3（记录驳回原因）。
// 返回是否更新到了签名记录（未知 SignId 返回 false 而非报错，避免重推）。
func (s *SmsChannelService) HandleSignStatusPush(upSignID, status, refuseReason string) (bool, error) {
	if upSignID == "" {
		return false, nil
	}
	sign, err := s.smsRepo.GetSignByUpSignID(upSignID)
	if err != nil {
		return false, err
	}
	if sign == nil {
		log.Printf("签名状态推送: 未找到签名记录 up_sign_id=%s，忽略", upSignID)
		return false, nil
	}

	newStatus := sign.Status
	resultMsg := sign.ResultMessage
	switch status {
	case "1":
		newStatus, resultMsg = 2, ""
	case "3":
		newStatus, resultMsg = 3, refuseReason
	}
	if newStatus == sign.Status && resultMsg == sign.ResultMessage {
		return false, nil
	}
	if err := s.smsRepo.UpdateSignStatusByID(sign.ID, newStatus, resultMsg); err != nil {
		return false, err
	}
	log.Printf("签名状态推送已更新 [sign_id=%d, up_sign_id=%s, status=%s]", sign.ID, upSignID, status)
	sign.Status, sign.ResultMessage = newStatus, resultMsg
	auditSign(sign, "callback", "sign_push")
	return true, nil
}

// HandleTemplateStatusPush 处理上游模板状态推送（回调，无签名验签）。
// 按上游模板 ID（template_id）关联本地模板；status=1 通过→1、3 驳回→2（记录驳回原因）。
// 待平台审核（3）的模板忽略推送：其最新内容尚未报备上游，推送结果属于旧内容。
// 返回是否更新到了模板记录（未知模板 ID 返回 false 而非报错，避免重推）。
func (s *SmsChannelService) HandleTemplateStatusPush(templateID, status, refuseReason string) (bool, error) {
	if templateID == "" {
		return false, nil
	}
	t, err := s.smsRepo.GetTemplateByUpTemplateID(templateID)
	if err != nil {
		return false, err
	}
	if t == nil {
		log.Printf("模板状态推送: 未找到模板记录 template_id=%s，忽略", templateID)
		return false, nil
	}
	if t.Status == model.SmsTemplateStatusPlatformReview {
		log.Printf("模板状态推送: 模板待平台审核，忽略推送 [template_id=%d, up_template_id=%s]", t.ID, templateID)
		return false, nil
	}

	newStatus := t.Status
	reason := t.Reason
	switch status {
	case "1":
		newStatus, reason = 1, ""
	case "3":
		newStatus, reason = 2, refuseReason
	}
	if newStatus == t.Status && reason == t.Reason {
		return false, nil
	}
	if err := s.smsRepo.UpdateTemplateStatusByID(t.ID, newStatus, reason); err != nil {
		return false, err
	}
	log.Printf("模板状态推送已更新 [template_id=%d, up_template_id=%s, status=%s]", t.ID, templateID, status)
	t.Status, t.Reason = newStatus, reason
	auditTemplate(t, "callback", "template_push")
	return true, nil
}

// FillTemplateID 回填模板 ID（上游控制台创建后填写）
func (s *SmsChannelService) FillTemplateID(userID, id int64, templateID string) error {
	return s.smsRepo.FillTemplateID(userID, id, templateID)
}

// HandleReplyPush 处理上游短信回复推送（回调，无签名验签）。
// 按 taskId 关联发送记录归属用户并落库（sequence_id 去重）；若该笔发送提供了 notify_url，
// 则主动 POST 通知下游。返回是否识别到了发送记录（未知 taskId 返回 false 而非报错，避免重推）。
func (s *SmsChannelService) HandleReplyPush(reply upstream.SmsReplyItem) (bool, error) {
	if reply.TaskId == "" {
		return false, nil
	}
	sendRec, err := s.smsRepo.GetSendRecordByMessageSid(reply.TaskId)
	if err != nil {
		return false, err
	}
	if sendRec == nil {
		log.Printf("短信回复推送: 未找到发送记录 taskId=%s，忽略", reply.TaskId)
		return false, nil
	}

	r := &model.SmsReply{
		UserID:      sendRec.UserID,
		TaskID:      reply.TaskId,
		Phone:       reply.Phone,
		SequenceID:  reply.SequenceId,
		ContentDown: reply.ContentDown,
		ContentUp:   reply.ContentUp,
		RespTime:    reply.Timestamp,
		Status:      reply.Status,
		Tag:         reply.Tag,
	}
	created, err := s.smsRepo.CreateReply(r)
	if err != nil {
		return false, err
	}

	// 发送时提供了 notify_url：主动推送回复给下游（重复推送去重后不重复通知）
	if created && sendRec.NotifyURL != "" {
		s.notifyReplyDownstream(sendRec, r)
	}
	return true, nil
}

// QueryReplies 查询用户上行回复（下游主动查询）。
// 提供 date 时先按日向上游拉取回复并落库（上游已拉取标记已读，重复查询由本地去重兜底），
// 再返回本用户回复记录（可按 taskId 过滤、分页）。
func (s *SmsChannelService) QueryReplies(userID int64, date, taskID string, page, pageSize int) ([]*model.SmsReply, int64, error) {
	if date != "" {
		s.syncRepliesByDate(date)
	}
	return s.smsRepo.ListRepliesByUser(userID, taskID, page, pageSize)
}

// syncRepliesByDate 按日期从上游拉取回复并落库（归属用户按 taskId 关联发送记录）。
func (s *SmsChannelService) syncRepliesByDate(date string) {
	if !s.upstreamAvailable() {
		return
	}
	items, err := s.upstream.PullReply(date, 1, 100)
	if err != nil {
		log.Printf("拉取短信回复失败 [date=%s]: %v", date, err)
		return
	}
	for _, item := range items {
		sendRec, err := s.smsRepo.GetSendRecordByMessageSid(item.TaskId)
		if err != nil || sendRec == nil {
			continue // 非本平台发送的任务或查询失败，跳过
		}
		_, _ = s.smsRepo.CreateReply(&model.SmsReply{
			UserID:    sendRec.UserID,
			TaskID:    item.TaskId,
			Phone:     item.Phone,
			ContentUp: item.RespContent,
			RespTime:  item.RespTime,
			Tag:       item.Tag,
		})
	}
}

// notifyReplyDownstream 将上行回复主动 POST 到下游 notify_url（携带 HMAC 签名，供下游校验防伪造）。
func (s *SmsChannelService) notifyReplyDownstream(rec *model.SmsSendRecord, r *model.SmsReply) {
	// 使用发起该次发送的 API 密钥 secret 生成签名（下游用同一 secret 校验）；
	// 控制台发起/历史记录未记录密钥（api_id=0）时回退到该账号主密钥
	sign := ""
	if cred := resolveRecordAPIKey(s.apiKeyRepo, rec.APIID, rec.UserID); cred != nil && cred.APISecret != "" {
		sign = buildSmsReplyNotifySign(cred.APISecret, rec.BizNo, rec.MessageSid, r.Phone, r.ContentDown, r.ContentUp, r.Status)
	}

	payload := map[string]interface{}{
		"biz_no":       rec.BizNo,
		"message_sid":  rec.MessageSid,
		"phone":        r.Phone,
		"content_down": r.ContentDown,
		"content_up":   r.ContentUp,
		"sequence_id":  r.SequenceID,
		"timestamp":    r.RespTime,
		"status":       r.Status,
		"tag":          r.Tag,
		"sign":         sign,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("序列化短信回复通知失败 [record_id=%d]: %v", rec.ID, err)
		return
	}

	// 通知重试以「业务单号」为键：回复用其序列号（推送场景），拉取场景无序列号时回退回复记录 ID
	replyBizNo := r.SequenceID
	if replyBizNo == "" {
		replyBizNo = strconv.FormatInt(r.ID, 10)
	}

	resp, err := http.Post(rec.NotifyURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("通知下游短信回复失败 [record_id=%d, url=%s]: %v", rec.ID, rec.NotifyURL, err)
		s.notifySvc.Enqueue("sms_reply", replyBizNo, rec.ID, rec.UserID, rec.NotifyURL, string(jsonData))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("通知下游短信回复成功 [record_id=%d, url=%s, status=%d]", rec.ID, rec.NotifyURL, resp.StatusCode)
	} else {
		log.Printf("通知下游短信回复返回异常 [record_id=%d, url=%s, status=%d]", rec.ID, rec.NotifyURL, resp.StatusCode)
		s.notifySvc.Enqueue("sms_reply", replyBizNo, rec.ID, rec.UserID, rec.NotifyURL, string(jsonData))
	}
}

// buildSmsReplyNotifySign 构造短信回复下游回调签名：
// 对固定字段按 key 字典序拼接为 k=v&k=v... 的原始字符串（不做 URL 编码），
// 再以 HMAC-SHA256(api_secret, canonical) 计算十六进制小写签名。
// 下游（如 zjmf_v10 插件）用相同算法与自己的 api_secret 校验，杜绝伪造回调。
func buildSmsReplyNotifySign(apiSecret, bizNo, messageSid, phone, contentDown, contentUp, status string) string {
	fields := map[string]string{
		"biz_no":       bizNo,
		"content_down": contentDown,
		"content_up":   contentUp,
		"message_sid":  messageSid,
		"phone":        phone,
		"status":       status,
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(fields[k])
	}

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(sb.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// ListSendRecords 查询发送记录（分页 + 日期筛选）
func (s *SmsChannelService) ListSendRecords(userID int64, startDate, endDate string, page, pageSize int) ([]*model.SmsSendRecord, int64, error) {
	return s.smsRepo.ListSendRecords(userID, startDate, endDate, page, pageSize)
}

// SMSStats 短信统计（总条数/总金额 + 按日趋势）
func (s *SmsChannelService) SMSStats(userID int64, days int) (*repository.SmsStatsResult, error) {
	return s.smsRepo.SmsStats(userID, days)
}

// ListSendRecordsForAdmin 后台全量发送记录（管理端，含用户手机号）
func (s *SmsChannelService) ListSendRecordsForAdmin(startDate, endDate string, page, pageSize int) ([]*repository.AdminSmsRecord, int64, error) {
	return s.smsRepo.ListAllSendRecords(startDate, endDate, page, pageSize)
}

// SMSStatsForAdmin 后台全量短信统计（管理端）
func (s *SmsChannelService) SMSStatsForAdmin(days int) (*repository.SmsStatsResult, error) {
	return s.smsRepo.SmsStatsAll(days)
}

// ListRepliesForAdmin 后台全量短信回复（管理端，含用户手机号）
func (s *SmsChannelService) ListRepliesForAdmin(page, pageSize int) ([]*repository.AdminSmsReply, int64, error) {
	return s.smsRepo.ListAllReplies(page, pageSize)
}

// PullReport 按上游任务ID（taskId）从上游拉取回执明细（下游查询回执用）。
func (s *SmsChannelService) PullReport(taskId string, pageNo, pageSize int) ([]upstream.SmsReportItem, error) {
	if !s.upstreamAvailable() {
		return nil, fmt.Errorf("短信通道未配置")
	}
	return s.upstream.PullReport(taskId, pageNo, pageSize)
}

// PullReportForUser 下游查询回执：先校验 taskId 归属本账号（发送记录 user_id 一致）再拉取，
// 防止凭已知 taskId 查询他人回执（含手机号）。
func (s *SmsChannelService) PullReportForUser(userID int64, taskId string, pageNo, pageSize int) ([]upstream.SmsReportItem, error) {
	rec, err := s.smsRepo.GetSendRecordByMessageSid(taskId)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.UserID != userID {
		return nil, fmt.Errorf("回执不存在")
	}
	return s.PullReport(taskId, pageNo, pageSize)
}

// HandleSmsReportPush 处理上游回执推送（短信回执推送，无签名验签）。
// 按 taskId 关联本地发送记录；任一条目 respCode != DELIVRD 视为该批发送失败（记录首个失败回执），
// 否则标记成功。落库后若发送时提供了 notify_url，则主动 POST 通知下游。
// 返回是否更新到了发送记录（未知 taskId 返回 false 而非报错，避免重推）。
func (s *SmsChannelService) HandleSmsReportPush(report upstream.SmsReportItem, taskId string) (bool, error) {
	if taskId == "" {
		return false, nil
	}
	rec, err := s.smsRepo.GetSendRecordByMessageSid(taskId)
	if err != nil {
		return false, err
	}
	if rec == nil {
		log.Printf("短信回执推送: 未找到发送记录 taskId=%s，忽略", taskId)
		return false, nil
	}

	// 单条回执内容（content）可能为空，仅按当前条目更新状态
	success := report.RespCode == "DELIVRD"
	failMsg := ""
	if !success && report.RespCode != "" {
		failMsg = fmt.Sprintf("%s %s", report.RespCode, report.CodeDesc)
	}

	// 回执失败：退还已扣费用。先把记录按「未计费」落库（金额/资源包条数清零），再执行退款，
	// 这样上游重推同一回执时不会重复退款（退款失败仅记日志，需人工补退）。
	var (
		refund     func() error
		refundNote string
	)
	if !success && rec.Status == 0 && (rec.PackCount > 0 || rec.Amount > 0) {
		payType, packID, packCount, amount := rec.PayType, rec.PackID, rec.PackCount, rec.Amount
		rec.Amount, rec.PackCount, rec.PackID = 0, 0, 0
		refundNote = fmt.Sprintf("pay_type=%d, pack_id=%d, pack_count=%d, amount=%.4f", payType, packID, packCount, amount)
		refund = func() error {
			return s.refundSms(rec.UserID, payType, packID, packCount, amount, rec.ID)
		}
	}
	if err := s.smsRepo.UpdateSendRecordReceipt(rec, success, failMsg); err != nil {
		return false, err
	}
	if refund != nil {
		if err := refund(); err != nil {
			log.Printf("短信回执失败退款失败（需人工补退）[record_id=%d, user_id=%d, %s]: %v", rec.ID, rec.UserID, refundNote, err)
		}
	}
	audit.SmsSendRecord("sms_receipt", "callback", rec,
		audit.KV("success", success), audit.KV("resp_code", report.RespCode),
		audit.KV("refund", refundNote), audit.KV("fail_message", failMsg))

	// 发送时提供了 notify_url：主动推送回执给下游（失败仅记录日志，不影响回执落库）
	if rec.NotifyURL != "" {
		s.notifySmsReceipt(rec, report, success, failMsg)
	}
	return true, nil
}

// notifySmsReceipt 将上游回执主动 POST 到下游 notify_url（携带 HMAC 签名，供下游校验防伪造）。
func (s *SmsChannelService) notifySmsReceipt(rec *model.SmsSendRecord, report upstream.SmsReportItem, success bool, failMsg string) {
	status := "success"
	if !success {
		status = "fail"
	}

	// 使用发起该次发送的 API 密钥 secret 生成签名（下游用同一 secret 校验）；
	// 控制台发起/历史记录未记录密钥（api_id=0）时回退到该账号主密钥
	sign := ""
	if cred := resolveRecordAPIKey(s.apiKeyRepo, rec.APIID, rec.UserID); cred != nil && cred.APISecret != "" {
		sign = buildSmsNotifySign(cred.APISecret, rec.BizNo, rec.MessageSid, report.Phone, report.RespCode, report.CodeDesc, status)
	}

	payload := map[string]interface{}{
		"biz_no":       rec.BizNo,
		"message_sid":  rec.MessageSid,
		"phone":        report.Phone,
		"status":       status,
		"resp_code":    report.RespCode,
		"code_desc":    report.CodeDesc,
		"resp_time":    report.RespTime,
		"sequence_id":  report.SequenceId,
		"fail_message": failMsg,
		"sign":         sign,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("序列化短信回执通知失败 [record_id=%d]: %v", rec.ID, err)
		return
	}

	resp, err := http.Post(rec.NotifyURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("通知下游短信回执失败 [record_id=%d, url=%s]: %v", rec.ID, rec.NotifyURL, err)
		s.notifySvc.Enqueue("sms_receipt", rec.BizNo, rec.ID, rec.UserID, rec.NotifyURL, string(jsonData))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("通知下游短信回执成功 [record_id=%d, url=%s, status=%d]", rec.ID, rec.NotifyURL, resp.StatusCode)
	} else {
		log.Printf("通知下游短信回执返回异常 [record_id=%d, url=%s, status=%d]", rec.ID, rec.NotifyURL, resp.StatusCode)
		s.notifySvc.Enqueue("sms_receipt", rec.BizNo, rec.ID, rec.UserID, rec.NotifyURL, string(jsonData))
	}
}

// buildSmsNotifySign 构造短信回执下游回调签名：
// 对固定字段按 key 字典序拼接为 k=v&k=v... 的原始字符串（不做 URL 编码），
// 再以 HMAC-SHA256(api_secret, canonical) 计算十六进制小写签名。
// 下游（如 zjmf_v10 插件）用相同算法与自己的 api_secret 校验，杜绝伪造回调。
func buildSmsNotifySign(apiSecret, bizNo, messageSid, phone, respCode, codeDesc, status string) string {
	fields := map[string]string{
		"biz_no":      bizNo,
		"code_desc":   codeDesc,
		"message_sid": messageSid,
		"phone":       phone,
		"resp_code":   respCode,
		"status":      status,
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(fields[k])
	}

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(sb.String()))
	return hex.EncodeToString(mac.Sum(nil))
}
