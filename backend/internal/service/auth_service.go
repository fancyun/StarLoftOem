package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"oemrpa/internal/audit"
	"oemrpa/internal/logstore"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/site"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

// StartAuthResult StartAuth 返回结果
// 账户实名（source=1）不产生认证记录，走 KycRecord；API 调用（source=2）走 Record
type StartAuthResult struct {
	Record    *model.AuthRecord
	KycRecord *model.KycPersonal
	AuthURL   string // 上游认证 URL
	Token     string // 上游 token
	BizID     string // 上游 biz_id
}

// freeFailLimit 账户实名（Web）免费认证次数上限（账号终身累计，写死为3次）
// 个人实名与企业实名各自独立计次：免费次数按已发起的核验次数统计（核验失败也占用免费次数）。
// 免费次数用尽后按次计费：个人 KYC_PERSONAL_PRICE（默认1元）、企业 KYC_ENTERPRISE_PRICE（默认2元）。
const freeFailLimit = 3

// 上游回调与跳转地址（写死路径，与后端回调路由/前端页面绑定）。
// 这些地址只在上游与平台之间传递（浏览器仅做瞬时 302 穿过）。
const (
	finAuthReturnURL = "/certification" // 上游同步跳转默认地址：未传入 return_url 时兜底，认证完成后返回账户实名页
)

// finAuthNotifyURL 上游（StarLoft）异步通知地址：认证结果由本平台接收并落地（API 域回调路由）
func finAuthNotifyURL() string {
	return site.Platform().APIBase() + "/v1/callback/starloft/fv"
}

// generateRecordBizNo 生成 kyc/kyb 实名记录的内部业务单号：{userID}{毫秒时间戳}。
// 区别于平台对用户调用（认证记录 auth_record）生成的 20 位随机 biz_no，本单号仅作实名记录自身唯一标识。
func generateRecordBizNo(userID int64) string {
	return fmt.Sprintf("%d%d", userID, time.Now().UnixMilli())
}

// AuthService 认证服务
type AuthService struct {
	// finAuth 人脸核验（FV）产品上游（StarLoft 平台开放 API）
	finAuth upstream.FinAuthInterface
	// faceProvider 账户实名人脸核身 provider（上游 StarLoft 平台 / 腾讯云可切换）
	faceProvider upstream.FaceProvider
	// enterpriseVerifier 企业工商四要素核验 provider（腾讯云 OCR / 阿里云可切换）
	enterpriseVerifier upstream.EnterpriseVerifier
	recordRepo         *repository.AuthRecordRepository // 人脸核验订单库（oem_fv）
	userRepo           *repository.UserRepository
	apiKeyRepo         *repository.ApiKeyRepository
	kycRecordRepo      *repository.KycPersonalRepository
	kybRepo            *repository.KybEnterpriseRepository
	resourcePackRepo   *repository.ResourcePackRepository
	balanceService     *BalanceService
	notifySvc          *NotifyService
	mediaDir           string  // 认证媒体根目录（照片/视频自动保存于此，独立于用户上传目录）
	fvAuthPrice        float64 // 下游有源人脸核验单价（元/次），存人脸核验产品库 product_config
	fvSelfPrice        float64 // 下游无源人脸核验单价（元/次），存人脸核验产品库 product_config
	kycPersonalPrice   float64 // 个人实名免费次数用尽后单价（元/次），存系统库设置表
	kycEnterprisePrice float64 // 企业实名免费次数用尽后单价（元/次），存系统库设置表
	// promotionService 推广服务：按用户适用单价计费，并按下级成交额计提提成
	promotionService *PromotionService
	// siteHosts 按用户归属解析站点域名组（推广 白标下发的对外地址以此为准）
	siteHosts func(userID int64) site.Hosts
}

// SetPromotionService 注入推广服务（在路由装配时调用，避免与计费服务的构造顺序耦合）
func (s *AuthService) SetPromotionService(o *PromotionService) {
	s.promotionService = o
}

// SetSiteHostsResolver 注入站点域名解析器（在路由装配时调用）
func (s *AuthService) SetSiteHostsResolver(fn func(userID int64) site.Hosts) {
	s.siteHosts = fn
}

// hostsFor 返回用户归属的站点域名组（未注入解析器时回落平台默认站点）
func (s *AuthService) hostsFor(userID int64) site.Hosts {
	if s.siteHosts == nil {
		return site.Platform()
	}
	return s.siteHosts(userID)
}

// NewAuthService 创建认证服务
func NewAuthService(
	finAuth upstream.FinAuthInterface,
	faceProvider upstream.FaceProvider,
	enterpriseVerifier upstream.EnterpriseVerifier,
	recordRepo *repository.AuthRecordRepository,
	userRepo *repository.UserRepository,
	apiKeyRepo *repository.ApiKeyRepository,
	kycRecordRepo *repository.KycPersonalRepository,
	kybRepo *repository.KybEnterpriseRepository,
	resourcePackRepo *repository.ResourcePackRepository,
	balanceService *BalanceService,
	notifySvc *NotifyService,
	mediaDir string,
	fvAuthPrice float64,
	fvSelfPrice float64,
	kycPersonalPrice float64,
	kycEnterprisePrice float64,
) *AuthService {
	return &AuthService{
		finAuth:            finAuth,
		faceProvider:       faceProvider,
		enterpriseVerifier: enterpriseVerifier,
		recordRepo:         recordRepo,
		userRepo:           userRepo,
		apiKeyRepo:         apiKeyRepo,
		kycRecordRepo:      kycRecordRepo,
		kybRepo:            kybRepo,
		resourcePackRepo:   resourcePackRepo,
		balanceService:     balanceService,
		notifySvc:          notifySvc,
		mediaDir:           mediaDir,
		fvAuthPrice:        fvAuthPrice,
		fvSelfPrice:        fvSelfPrice,
		kycPersonalPrice:   kycPersonalPrice,
		kycEnterprisePrice: kycEnterprisePrice,
	}
}

// recordRepoForProduct 按产品标识返回对应认证记录仓库（人脸核验服务归 FV 库）。
func (s *AuthService) recordRepoForProduct(product string) *repository.AuthRecordRepository {
	return s.recordRepo
}

// recordRepoOf 按订单归属返回对应仓库
func (s *AuthService) recordRepoOf(record *model.AuthRecord) *repository.AuthRecordRepository {
	return s.recordRepoForProduct(record.Product)
}

// GetFreeAuthRemaining 返回个人实名（source=1）剩余免费认证次数（按已发起核验次数计，扣除重置基准偏移）
func (s *AuthService) GetFreeAuthRemaining(userID int64) (int, error) {
	attempts, err := s.kycRecordRepo.CountUserKycAttempts(userID)
	if err != nil {
		return 0, err
	}
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return 0, err
	}
	remaining := freeFailLimit - (attempts - user.PersonalFreeBase)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// GetKybFreeAuthRemaining 返回企业实名剩余免费认证次数（按已发起核验次数计，扣除重置基准偏移）
func (s *AuthService) GetKybFreeAuthRemaining(userID int64) (int, error) {
	attempts, err := s.kybRepo.CountUserKybAttempts(userID)
	if err != nil {
		return 0, err
	}
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return 0, err
	}
	remaining := freeFailLimit - (attempts - user.EnterpriseFreeBase)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// ResetRealnameFreeBase 管理员重置用户免费实名次数：把基准设为当前已发起核验次数，剩余免费次数恢复 freeFailLimit。
// kind 取值：personal-个人实名 enterprise-企业实名。
// 返回重置后的对应类型剩余免费次数；未重置的类型返回 -1 表示不变。
func (s *AuthService) ResetRealnameFreeBase(userID int64, kind string) (personalRemaining, enterpriseRemaining int, err error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return 0, 0, err
	}
	personalBase, enterpriseBase := user.PersonalFreeBase, user.EnterpriseFreeBase
	switch kind {
	case "personal":
		att, err := s.kycRecordRepo.CountUserKycAttempts(userID)
		if err != nil {
			return 0, 0, err
		}
		personalBase = att
	case "enterprise":
		att, err := s.kybRepo.CountUserKybAttempts(userID)
		if err != nil {
			return 0, 0, err
		}
		enterpriseBase = att
	default:
		return 0, 0, fmt.Errorf("不支持的实名类型")
	}
	if err := s.userRepo.UpdateRealnameFreeBase(userID, personalBase, enterpriseBase); err != nil {
		return 0, 0, err
	}
	personalRemaining, enterpriseRemaining = -1, -1
	if kind == "personal" {
		personalRemaining = freeFailLimit
	}
	if kind == "enterprise" {
		enterpriseRemaining = freeFailLimit
	}
	return personalRemaining, enterpriseRemaining, nil
}

// StartAuth 发起认证
// source: 1-账户实名（Web）
//   - 账户实名（source=1）：认证信息单独储存在系统库实名记录表（kyc）中；
//     免费次数（3次）内不产生认证记录、不计费，用尽后按 KYC_PERSONAL_PRICE 计费。
func (s *AuthService) StartAuth(
	userID int64,
	product, name, idCard, returnURL, notifyURL, bizExtraData string,
	source int,
	free bool,
) (*StartAuthResult, error) {
	// return_url 未传入时使用默认跳转地址（认证完成后返回账户实名页）
	if returnURL == "" {
		returnURL = finAuthReturnURL
	}

	// 仅支持账户实名（source=1）
	if source != 1 {
		return nil, fmt.Errorf("不支持的认证类型")
	}
	return s.startAccountAuth(userID, name, idCard, returnURL, notifyURL, bizExtraData)
}

// startAccountAuth 账户实名（source=1）发起认证：改用腾讯云人脸核身（个人实名）。
// 实名信息储存在系统库实名记录表（kyc）。免费次数（3次）内不计费；
// 免费次数用尽后按 KYC_PERSONAL_PRICE 计费（走订单：优先资源包、再扣余额）。
// 流程：创建实名记录 → 计费 → 腾讯云 DetectAuth 发起人脸核身（姓名+身份证号核验+活体），
// 返回 H5 核身地址；完成核身后跳回实名页，由 SyncKycRecord 查询核身结果（GetDetectInfoEnhanced）并落地。
func (s *AuthService) startAccountAuth(
	userID int64,
	name, idCard, returnURL, notifyURL, bizExtraData string,
) (*StartAuthResult, error) {
	if s.faceProvider == nil {
		return nil, errors.New("人脸核身服务未配置")
	}

	// 账户实名无认证记录，实名记录单号即 {userID}{毫秒时间戳}
	bizNo := generateRecordBizNo(userID)

	// 免费次数按「本次之前已发起的核验次数」计：须在创建本次记录前统计，
	// 否则本次记录被 COUNT 计入而少免一次（与 KYB 的统计时机保持一致）。
	attempts, err := s.kycRecordRepo.CountUserKycAttempts(userID)
	if err != nil {
		return nil, fmt.Errorf("查询个人实名核验次数失败: %w", err)
	}

	// 创建实名记录（状态：认证中）
	kycRecord := &model.KycPersonal{
		UserID:       userID,
		BizNo:        bizNo,
		ReturnURL:    returnURL,
		NotifyURL:    notifyURL,
		BizExtraData: bizExtraData,
		Name:         name,
		IDCard:       idCard,
		Status:       1,
	}
	if err := s.kycRecordRepo.Create(kycRecord); err != nil {
		return nil, fmt.Errorf("create kyc record failed: %w", err)
	}
	audit.KycRecord("kyc_create", "console", kycRecord,
		audit.KV("return_url", returnURL), audit.KV("notify_url", notifyURL))

	// 免费次数用尽后计费（直接记账到系统库统一账单，绑定本实名记录）
	payType, packID, err := s.chargeKycAttempt(userID, attempts, model.ServiceKYCPersonal, s.platformScopedPrice(userID, model.ServiceKYCPersonal, s.getKycPersonalPrice()), kycRecord.ID, "kyc")
	if err != nil {
		_ = s.kycRecordRepo.UpdateResult(kycRecord.ID, 3, "DEDUCT_FAILED", "扣费失败", "", nil)
		audit.KycRecord("kyc_charge_failed", "console", kycRecord, audit.KV("reason", "扣费失败"))
		return nil, err
	}

	// 发起腾讯云人脸核身（姓名 + 身份证号核验 + 活体检测），返回 H5 核身地址
	detect, err := s.faceProvider.StartAuth(name, idCard, s.hostsFor(userID).ConsoleBase()+"/certification", bizNo)
	if err != nil {
		log.Printf("获取腾讯云人脸核身 Token 失败: %v", err)
		// 已扣费则原路退还（免费次数内不计费），并将实名记录标记为失败（连接超时）
		s.refundKycCharge(userID, payType, packID, s.platformScopedPrice(userID, model.ServiceKYCPersonal, s.getKycPersonalPrice()), kycRecord.ID, "kyc")
		_ = s.kycRecordRepo.UpdateResult(kycRecord.ID, 3, "TIMEOUT", "连接超时", "", nil)
		kycRecord.Status, kycRecord.ResultCode, kycRecord.ResultMessage = 3, "TIMEOUT", "连接超时"
		audit.KycRecord("kyc_start_failed", "console", kycRecord, audit.KV("reason", "获取腾讯云 Token 失败"))
		return nil, fmt.Errorf("detect auth failed: %w", err)
	}

	// 写入记录上游信息（BizToken 供后续查询核身结果）
	if err := s.kycRecordRepo.UpdateUpstreamInfo(kycRecord.ID, detect.Token, "", detect.Token); err != nil {
		log.Printf("更新实名记录上游信息失败 [record_id=%d]: %v", kycRecord.ID, err)
		// 已扣费则原路退还（与 DetectAuth 失败分支一致，避免用户白扣费）
		s.refundKycCharge(userID, payType, packID, s.platformScopedPrice(userID, model.ServiceKYCPersonal, s.getKycPersonalPrice()), kycRecord.ID, "kyc")
		_ = s.kycRecordRepo.UpdateResult(kycRecord.ID, 3, "TIMEOUT", "写入token失败", "", nil)
		kycRecord.Status, kycRecord.ResultCode, kycRecord.ResultMessage = 3, "TIMEOUT", "写入token失败"
		audit.KycRecord("kyc_start_failed", "console", kycRecord, audit.KV("reason", "写入 token 失败"))
		return nil, fmt.Errorf("update kyc record upstream info failed: %w", err)
	}
	kycRecord.UpToken = detect.Token
	audit.KycRecord("kyc_upstream_token", "console", kycRecord, audit.KV("token", detect.Token))

	// 拼接 H5 核身地址：Url?token=BizToken
	return &StartAuthResult{
		KycRecord: kycRecord,
		AuthURL:   detect.AuthURL,
		Token:     detect.Token,
	}, nil
}

// getKycPersonalPrice 个人实名超次单价（KYC_PERSONAL_PRICE），未配置时兜底 1.00
func (s *AuthService) getKycPersonalPrice() float64 {
	if s.kycPersonalPrice > 0 {
		return s.kycPersonalPrice
	}
	return 1.00
}

// getKycEnterprisePrice 企业实名超次单价（KYC_ENTERPRISE_PRICE），未配置时兜底 2.00
func (s *AuthService) getKycEnterprisePrice() float64 {
	if s.kycEnterprisePrice > 0 {
		return s.kycEnterprisePrice
	}
	return 2.00
}

// GetKycPersonalPriceForDisplay 个人实名超次单价（前端展示用）
func (s *AuthService) GetKycPersonalPriceForDisplay() float64 {
	return s.getKycPersonalPrice()
}

// GetKycEnterprisePriceForDisplay 企业实名超次单价（前端展示用）
func (s *AuthService) GetKycEnterprisePriceForDisplay() float64 {
	return s.getKycEnterprisePrice()
}

// chargeKycAttempt 实名（Web）免费次数用尽后计费：不再创建 fv 认证记录，
// 直接扣费并写入系统库统一账单（bill），账单通过 refType 绑定对应实名记录（kyc/kyb）。
// attempts 为已发起的核验次数（含本次前）；不足免费上限返回 payType=0 表示免费（不扣费）。
// 余额不足返回 ErrInsufficientBalance（调用方负责将实名记录标记失败）。
// 返回实际计费方式（0-免费 1-余额 2-资源包）与命中的资源包 ID，供上游失败时原路退还。
// 账户实名属于平台账户体系能力（登录验证码、人脸核身、实名核验的公开成本均由平台承担），
// 不参与推广提成：推广用户也按平台统一价格计费（平台给单个用户的定向定价仍有效）。
func (s *AuthService) chargeKycAttempt(userID int64, attempts int, product string, price float64, recordID int64, recordTable string) (int, int64, error) {
	if attempts < freeFailLimit {
		return 0, 0, nil
	}
	payType, packID, err := s.balanceService.DeductProductFee(userID, product, product, price, recordID, "实名超次计费", recordTable)
	if err != nil {
		return 0, 0, err
	}
	return payType, packID, nil
}

// refundKycCharge 退还实名超次扣费（上游调用失败时调用，免费流程不涉及）
func (s *AuthService) refundKycCharge(userID int64, payType int, packID int64, price float64, recordID int64, recordTable string) {
	if payType == 0 || price <= 0 {
		return
	}
	if err := s.balanceService.RefundProductFee(userID, price, int64(payType), packID, recordID, "实名核验失败退款", recordTable); err != nil {
		log.Printf("实名核验失败退款失败 [user_id=%d, record_id=%d]: %v", userID, recordID, err)
	}
}

// priceOf 返回用户适用的服务单价：用户级定向定价优先，无覆盖回落平台价
func (s *AuthService) priceOf(userID int64, service string, platform float64) float64 {
	if s.promotionService != nil {
		if p := s.promotionService.UnitPrice(userID, service); p > 0 {
			return p
		}
	}
	return platform
}

// platformScopedPrice 账户实名等平台承担成本的服务单价：仅应用平台给单个用户的定向定价，
// 不沿 推广 链（这些服务不进入 推广 结算，故 推广 定价对其无效）
func (s *AuthService) platformScopedPrice(userID int64, service string, platform float64) float64 {
	if s.promotionService != nil {
		if p := s.promotionService.UserScopedUnitPrice(userID, service); p > 0 {
			return p
		}
	}
	return platform
}

// chargeRecordAfterUpstream 上游已接受本次核验后扣费（资源包优先、余额兜底）并回写订单计费信息：
// 资源包扣量时订单金额记 0（另记扣减次数 pack_count），余额支付时金额为单价。
// 扣费失败不回滚已发起的核验，仅记录日志便于人工对账（发起前已校验过额度）。
func (s *AuthService) chargeRecordAfterUpstream(record *model.AuthRecord, userPack *model.UserResourcePack, userID int64, price float64) {
	// 推广 计费通道：按用户适用单价扣费（推广 用户消费结算给直接上级），失败保持未计费以便人工对账
	if s.promotionService != nil {
		payType, packID, amount, err := s.promotionService.ChargeUnitFee(userID, record.Product, record.Product, 1, record.ID, "认证扣费", "auth_record", model.CommissionBizFvAuth)
		if err != nil {
			log.Printf("扣除费用失败（核验已发起，需人工对账）[record_id=%d]: %v", record.ID, err)
			payType, packID, amount = 0, 0, 0
		}
		packCount := 0
		if payType == model.PayTypePack {
			packCount = 1
		}
		record.Cost, record.PayType, record.UserPackID, record.PackCount = amount, payType, packID, packCount
		if err := s.recordRepoOf(record).UpdateRecordCharge(record.ID, amount, payType, packID, packCount); err != nil {
			log.Printf("回写订单计费信息失败 [record_id=%d]: %v", record.ID, err)
		}
		audit.AuthRecord("record_charge", "api", record, audit.KV("channel", "promotion"))
		return
	}

	cost, payType, packID, packCount := 0.0, record.PayType, int64(0), 0
	switch {
	case record.PayType == 2 && userPack != nil:
		ok, err := s.resourcePackRepo.DeductUserPackCount(userPack.ID, userID)
		if err != nil {
			log.Printf("扣减资源包失败 [record_id=%d, pack_id=%d]: %v", record.ID, userPack.ID, err)
			payType = 0
		} else if ok {
			// 资源包扣量不写统一账单（bill 只记资金变动），扣减次数记录在业务表（auth_record.pack_count）
			packID, packCount = userPack.ID, 1
		} else {
			// 资源包并发耗尽/无剩余次数，回退到余额扣费
			log.Printf("资源包已耗尽，回退余额扣费 [record_id=%d, pack_id=%d]", record.ID, userPack.ID)
			payType = 1
		}
	case record.PayType == 1:
		payType = 1
	default:
		// 免费/其他：不计费（认证记录 pay_type：0-免费 1-余额 2-资源包）
		payType = 0
	}

	if payType == 1 {
		if err := s.balanceService.DeductBalance(userID, price, record.ID, "认证扣费", productOf(record.Product), record.Product, "auth_record"); err != nil {
			log.Printf("扣除余额失败（核验已发起，需人工对账）[record_id=%d]: %v", record.ID, err)
			record.Cost, record.PackCount = 0, 0
		} else {
			cost = price
		}
	}

	record.Cost, record.PayType, record.UserPackID, record.PackCount = cost, payType, packID, packCount
	if err := s.recordRepoOf(record).UpdateRecordCharge(record.ID, cost, payType, packID, packCount); err != nil {
		log.Printf("回写订单计费信息失败 [record_id=%d]: %v", record.ID, err)
	}
	audit.AuthRecord("record_charge", "api", record)
}

// refundRecordCharge 退还未计费订单的费用（资源包退扣减次数，余额按实际扣费金额退还）；
// 未实际扣费（无扣减次数且金额为 0）时不动作。
func (s *AuthService) refundRecordCharge(record *model.AuthRecord, remark string) error {
	if s.promotionService != nil {
		if record.PackCount == 0 && record.Cost <= 0 {
			return nil
		}
		return s.promotionService.RefundUnitFee(record.UserID, record.Product, record.PayType, record.UserPackID, record.Cost, 1, record.ID, remark, "auth_record")
	}
	if record.PackCount > 0 && record.UserPackID > 0 {
		return s.resourcePackRepo.RefundUserPackCount(record.UserPackID)
	}
	if record.PayType == 1 && record.Cost > 0 {
		return s.balanceService.RefundBalance(record.UserID, record.Cost, record.ID, remark, "auth_record")
	}
	return nil
}

// hasCharged 判断订单是否已实际扣费（资源包扣次数或余额扣金额）
func hasCharged(record *model.AuthRecord) bool {
	return record.PackCount > 0 || (record.PayType == 1 && record.Cost > 0)
}

// getFvPrice 获取下游人脸核验单价：有源 fv_auth 取 fv_auth_price、无源 fv_self 取 fv_self_price
// （均在「人脸核验 → 产品配置」维护），未配置时兜底 1.00。
func (s *AuthService) getFvPrice(product string) float64 {
	if product == model.ServiceFVAuth && s.fvAuthPrice > 0 {
		return s.fvAuthPrice
	}
	if product == model.ServiceFVSelf && s.fvSelfPrice > 0 {
		return s.fvSelfPrice
	}
	return 1.00
}

// StartFvAuth 发起人脸核验认证（有源 fv_auth / 无源 fv_self）：
// 与 StartAuth 走同一套「订单+计费+上游 get_token」链路，但：
//   - record.Product 记录具体服务（fv_auth/fv_self），资源包按该服务隔离匹配
//   - 上游 return_url 指向本平台自站 return 回调端点，返回给下游的是本平台自站链接
//     （PC 展示二维码扫码核身、移动端 302 上游），核身后先回平台校对再跳下游
//   - return_url 可为空：为空时核身完成后不回跳，停留平台承接页展示认证结果
func (s *AuthService) StartFvAuth(userID, apiID int64, product, name, idCard, returnURL, notifyURL, bizExtraData string) (*StartAuthResult, error) {
	if product != model.ServiceFVAuth && product != model.ServiceFVSelf {
		return nil, fmt.Errorf("不支持的人脸核验服务: %s", product)
	}

	bizNo := utils.GenerateRandomDigits(20)

	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	// 计费：按产品区分单价（有源 fv_auth / 无源 fv_self 价格可不同），推广/单用户定价优先
	price := s.priceOf(userID, product, s.getFvPrice(product))

	var userPack *model.UserResourcePack
	payType := 0 // 0-免费 1-余额 2-资源包
	userPack, err = s.resourcePackRepo.GetUserActivePack(userID, product)
	if err != nil {
		return nil, fmt.Errorf("查询用户资源包失败: %w", err)
	}
	if userPack != nil {
		payType = 2
	} else {
		payType = 1
		if user.Balance < price {
			return nil, fmt.Errorf("余额不足，需要%.2f元，当前余额%.2f元", price, user.Balance)
		}
	}

	// 下游 API 调用不写实名记录表：核验信息与结果由认证记录（oem_fv.auth_record）单独承载
	record := &model.AuthRecord{
		BizNo:        bizNo,
		UserID:       userID,
		APIID:        apiID, // 发起该订单的 API 密钥（下游回调按此密钥签名；控制台发起为 0）
		Name:         name,
		IDCard:       idCard,
		ReturnURL:    returnURL,
		NotifyURL:    notifyURL,
		BizExtraData: bizExtraData,
		Cost:         0, // 实际扣费金额在上游接受核验后回写（资源包扣量记 0）
		PayType:      payType,
		Product:      product,
		Status:       0, // 待认证
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if payType == 2 {
		record.UserPackID = userPack.ID
	}
	if err := s.recordRepo.CreateRecord(record); err != nil {
		return nil, fmt.Errorf("create record failed: %w", err)
	}
	audit.AuthRecord("record_create", "api", record, audit.KV("notify_url", record.NotifyURL), audit.KV("return_url", record.ReturnURL))
	// 建单时不扣费：上游返回 token（接受本次核验）成功后才实际扣费，
	// 被上游拦截/连接失败的发起不产生扣费（见下方 chargeRecordAfterUpstream）。

	// 上游 return_url 指向本平台 API 域 return 回调端点：核身后先回平台校对，再 302 下游
	siteReturn := site.Platform().APIBase() + "/v1/fv/return"
	req := &upstream.GetTokenRequest{
		ReturnURL:      siteReturn,
		NotifyURL:      finAuthNotifyURL(),
		BizNo:          bizNo,
		Product:        product,
		ComparisonType: "1", // 人脸核验模式
		UUID:           fmt.Sprintf("%d", userID),
		IDCardMode:     "0", // 直接传入姓名与证件号
		IDCardName:     name,
		IDCardNumber:   idCard,
		BizExtraData:   bizExtraData,
	}

	tokenResp, err := s.finAuth.GetToken(req)
	if err != nil {
		log.Printf("获取 FinAuth Token 失败: %v", err)
		s.markRecordStartFailed(record)
		return nil, fmt.Errorf("get token failed: %w", err)
	}

	// 上游 token 到期时间（expired_time）：用于对外返回链接有效期与承接页倒计时
	var tokenExpireAt *time.Time
	if tokenResp.ExpiredTime > 0 {
		expireAt := time.Unix(tokenResp.ExpiredTime, 0)
		tokenExpireAt = &expireAt
		record.TokenExpireAt = tokenExpireAt
	}
	if err := s.recordRepo.UpdateRecordUpstreamInfo(record.ID, tokenResp.Token, tokenResp.BizID, tokenResp.RequestID, tokenExpireAt); err != nil {
		log.Printf("更新订单上游信息失败 [record_id=%d]: %v", record.ID, err)
		s.markRecordStartFailed(record)
		return nil, fmt.Errorf("update record upstream info failed: %w", err)
	}

	// 上游已接受本次核验：此时扣费（资源包优先、余额兜底），并回写订单计费信息
	s.chargeRecordAfterUpstream(record, userPack, userID, price)
	audit.AuthRecord("record_upstream_token", "api", record,
		audit.KV("token_expire_at", tokenExpireAtStr(record.TokenExpireAt)))

	siteURL := s.buildFvURL(userID, product, tokenResp.Token, tokenResp.ExpiredTime)

	return &StartAuthResult{
		Record:  record,
		AuthURL: siteURL,
		Token:   tokenResp.Token,
		BizID:   tokenResp.BizID,
	}, nil
}

// tokenExpireAtStr 上游 token 到期时间的审计文本（未返回时为空）
func tokenExpireAtStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// buildFvURL 构造人脸核验自站跳转链接（PC：自站页面渲染二维码；移动端：页面内 302 上游）
// 承接站点按发起用户归属的 推广 白标域名下发；expireAt 为上游返回的 token 到期时间戳
// （Unix 秒，0 表示未知），承接页据此显示倒计时。
func (s *AuthService) buildFvURL(userID int64, product, token string, expireAt int64) string {
	path := "fv"
	switch product {
	case model.ServiceFVAuth:
		path = "fv/auth"
	case model.ServiceFVSelf:
		path = "fv/self"
	}
	url := fmt.Sprintf("%s/%s?token=%s", s.hostsFor(userID).ServiceBase(), path, token)
	if expireAt > 0 {
		url += fmt.Sprintf("&expire_at=%d", expireAt)
	}
	return url
}

// HandleFvReturn 处理 FV 自站 return 回调：依据 biz_id/token 查订单，
// 主动向上游 get_result 校对一次结果（复用 syncRecordResult 落地订单、通知下游），
// 再返回下游 return_url 供 302 跳转（return_url 为空时回本平台承接页展示结果）。
func (s *AuthService) HandleFvReturn(bizID, token string) (string, error) {
	var record *model.AuthRecord
	var err error
	if bizID != "" {
		record, err = s.recordRepo.GetRecordByUpBizID(bizID)
	} else {
		record, err = s.recordRepo.GetRecordByUpToken(token)
	}
	if err != nil {
		return "", fmt.Errorf("record not found: %w", err)
	}

	// 主动校对一次上游结果（内部处理订单落地与下游通知）。
	// 注意：上游 get_result 每单仅允许 3 次，而回跳页可被用户反复刷新，
	// 因此只在「本单从未查过上游」时校对一次，后续刷新不再消耗查询额度。
	if record.UpQueryCount < 1 {
		_ = s.syncRecordResult(record, "return", false)
	}

	latest, err := s.recordRepo.GetRecordByBizNo(record.BizNo)
	if err == nil {
		record = latest
	}
	if record.ReturnURL == "" {
		// 下游未提供回跳地址：回到本平台承接页，由页面按订单状态展示认证结果
		if record.UpToken != "" {
			return s.buildFvURL(record.UserID, record.Product, record.UpToken, 0), nil
		}
		return s.hostsFor(record.UserID).ConsoleBase() + "/user/fv", nil
	}
	return record.ReturnURL, nil
}

// GetFvRecordByToken 按上游核身 token 查询认证记录（承接页状态轮询用）。
// 承接页每 3 秒轮询一次，这里只在「订单无 notify_url、结果只能靠拉取」时按额度与最小间隔
// 代查上游（见 syncResultOnPoll），保证无回调订单也能在核身完成后落地结果。
// force 表示用户已明确完成核身（承接页「我已完成核身」按钮），跳过最小间隔立即校对一次
func (s *AuthService) GetFvRecordByToken(token string, force bool) (*model.AuthRecord, error) {
	record, err := s.recordRepo.GetRecordByUpToken(token)
	if err != nil {
		return nil, err
	}
	s.syncResultOnPoll(record, "poll", force)
	if latest, err := s.recordRepo.GetRecordByID(record.ID); err == nil {
		return latest, nil
	}
	return record, nil
}

// GetUserByID 查询用户
func (s *AuthService) GetUserByID(userID int64) (*model.User, error) {
	return s.userRepo.GetUserByID(userID)
}

// GetLatestKycRecord 获取用户最新 KYC 记录
func (s *AuthService) GetLatestKycRecord(userID int64) (*model.KycPersonal, error) {
	return s.kycRecordRepo.GetLatestByUserID(userID)
}

// StartKybAuth 发起企业实名（自助）：
// 流程：1) 营业执照四要素核验（腾讯云 OCR VerifyBizLicenseEnterprise4，失败即终止并占用免费次数）；
//  2. 四要素通过后由法人本人进行人脸核身（腾讯云慧眼 DetectAuth），返回 H5 核身地址。
//
// 企业实名信息在法人扫脸通过后落地 user 表（realname_status=2）。
// 免费次数（3次）内不计费；用尽后按 KYC_ENTERPRISE_PRICE 计费（走订单：优先资源包、再扣余额）。
func (s *AuthService) StartKybAuth(
	userID int64,
	companyName, creditCode, legalName, legalIDCard, returnURL string,
) (*StartAuthResult, error) {
	// 已企业实名则无需重复认证
	if user, err := s.userRepo.GetUserByID(userID); err == nil && user.RealnameStatus == model.RealnameEnterprise {
		return nil, errors.New("已企业实名，无需重复认证")
	}
	if s.enterpriseVerifier == nil {
		return nil, errors.New("营业执照四要素核验服务未配置")
	}
	if s.faceProvider == nil {
		return nil, errors.New("人脸核身服务未配置")
	}

	bizNo := generateRecordBizNo(userID) // 企业实名无认证记录，记录单号即 {userID}{毫秒时间戳}

	// 免费次数用尽后计费（走订单：优先资源包、再扣余额）；扣费失败将企业实名记录标记为失败
	attempts, err := s.kybRepo.CountUserKybAttempts(userID)
	if err != nil {
		return nil, fmt.Errorf("查询企业实名核验次数失败: %w", err)
	}

	// 创建企业实名记录（状态：待四要素核验）
	rec := &model.KybEnterprise{
		UserID:           userID,
		BizNo:            bizNo,
		CompanyName:      companyName,
		CreditCode:       creditCode,
		LegalName:        legalName,
		LegalIDCard:      legalIDCard,
		Source:           0, // 自助
		FourFactorStatus: 0,
		Status:           0,
	}
	if err := s.kybRepo.Create(rec); err != nil {
		return nil, fmt.Errorf("create kyc enterprise record failed: %w", err)
	}
	audit.KybRecord("kyb_create", "console", rec)

	markKybFailed := func() {
		_ = s.kybRepo.UpdateResult(rec.ID, 3, "DEDUCT_FAILED", "扣费失败", "", nil)
	}
	// 免费次数用尽后计费（直接记账到系统库统一账单，绑定本企业实名记录）；账户实名由平台承担成本，不沿 推广 链定价
	kybPrice := s.platformScopedPrice(userID, model.ServiceKYCEnterprise, s.getKycEnterprisePrice())
	payType, packID, err := s.chargeKycAttempt(userID, attempts, model.ServiceKYCEnterprise, kybPrice, rec.ID, "kyb")
	if err != nil {
		markKybFailed()
		return nil, err
	}

	// 第一步：营业执照四要素核验（企业名称 + 统一社会信用代码 + 法人姓名 + 法人身份证号）
	four, err := s.enterpriseVerifier.Verify(companyName, creditCode, legalName, legalIDCard)
	if err != nil {
		// 上游异常：已扣费则原路退还，核验终止
		log.Printf("企业四要素核验上游调用失败 [user_id=%d, biz_no=%s]: %v", userID, bizNo, err)
		s.refundKycCharge(userID, payType, packID, kybPrice, rec.ID, "kyb")
		_ = s.kybRepo.UpdateFourFactor(rec.ID, 2, "")
		_ = s.kybRepo.UpdateResult(rec.ID, 3, "FOUR_FACTOR_ERROR", "企业信息核验异常，请稍后重试", "", nil)
		rec.Status, rec.ResultCode, rec.ResultMessage = 3, "FOUR_FACTOR_ERROR", "企业信息核验异常"
		audit.KybRecord("kyb_land", "console", rec, audit.KV("stage", "four_factor"))
		return nil, fmt.Errorf("企业信息核验异常，请稍后重试")
	}
	if !four.Matched {
		// 四要素不一致：核验不通过，占用一次核验次数
		_ = s.kybRepo.UpdateFourFactor(rec.ID, 2, four.RawData)
		_ = s.kybRepo.UpdateResult(rec.ID, 3, "FOUR_FACTOR_MISMATCH", "企业信息核验未通过，请核对企业名称、统一社会信用代码、法人信息后重试", four.RawData, nil)
		rec.Status, rec.ResultCode = 3, "FOUR_FACTOR_MISMATCH"
		audit.KybRecord("kyb_land", "console", rec, audit.KV("stage", "four_factor"))
		return nil, errors.New("企业信息核验未通过，请核对企业名称、统一社会信用代码、法人信息后重试")
	}
	// 四要素核验通过：记录结果，状态置「待法人扫脸」
	if err := s.kybRepo.UpdateFourFactor(rec.ID, 1, four.RawData); err != nil {
		return nil, fmt.Errorf("update four factor failed: %w", err)
	}
	_ = s.kybRepo.UpdateResult(rec.ID, 1, "0", "企业信息核验通过", four.RawData, nil)
	rec.Status, rec.ResultCode, rec.FourFactorStatus = 1, "0", 1
	audit.KybRecord("kyb_land", "console", rec, audit.KV("stage", "four_factor"))

	// 第二步：法人本人人脸核身高并发起（转向 H5 核身地址，完成核身后跳回企业实名页同步结果）
	detect, err := s.faceProvider.StartAuth(legalName, legalIDCard, s.hostsFor(userID).ConsoleBase()+"/certification", bizNo)
	if err != nil {
		s.refundKycCharge(userID, payType, packID, kybPrice, rec.ID, "kyb")
		_ = s.kybRepo.UpdateResult(rec.ID, 3, "TIMEOUT", "连接超时", "", nil)
		rec.Status, rec.ResultCode, rec.ResultMessage = 3, "TIMEOUT", "连接超时"
		audit.KybRecord("kyb_start_failed", "console", rec, audit.KV("stage", "face_detect"))
		return nil, fmt.Errorf("detect auth failed: %w", err)
	}
	// 存储 BizToken 到记录中，供后续查询结果
	if err := s.kybRepo.UpdateUpstreamInfo(rec.ID, detect.Token, "", detect.Token); err != nil {
		// 已扣费则原路退还（与 DetectAuth 失败分支一致，避免用户白扣费）
		s.refundKycCharge(userID, payType, packID, kybPrice, rec.ID, "kyb")
		_ = s.kybRepo.UpdateResult(rec.ID, 3, "TIMEOUT", "写入token失败", "", nil)
		return nil, fmt.Errorf("update kyc enterprise upstream info failed: %w", err)
	}
	rec.UpToken = detect.Token
	audit.KybRecord("kyb_upstream_token", "console", rec)

	// 拼接 H5 核身地址：Url?token=BizToken
	return &StartAuthResult{
		AuthURL: detect.AuthURL,
		Token:   detect.Token,
	}, nil
}

// GetKybRecord 获取用户最新企业实名记录
func (s *AuthService) GetKybRecord(userID int64) (*model.KybEnterprise, error) {
	return s.kybRepo.GetLatestByUserID(userID)
}

// BuildKybAuthURL 企业实名记录为「待法人扫脸」时返回继续认证地址。
// 腾讯云人脸核身为一次会话，中途跳出后需重新发起，故不提供继续地址。
func (s *AuthService) BuildKybAuthURL(rec *model.KybEnterprise) string {
	return ""
}

// SyncKybResult 同步最新企业实名记录的法人扫脸结果（腾讯云人脸核身 GetDetectInfo）：
// 成功（ErrCode==0）落地 user 表实名；返回终态失败则落失败；未完成保持待法人扫脸。
func (s *AuthService) SyncKybResult(userID int64) error {
	if s.faceProvider == nil {
		return nil
	}
	rec, err := s.kybRepo.GetLatestByUserID(userID)
	if err != nil || rec == nil || rec.Status != 1 || rec.UpToken == "" {
		return nil
	}
	result, err := s.faceProvider.QueryResult(rec.UpToken)
	if err != nil {
		log.Printf("同步企业实名扫脸结果失败 [user_id=%d, record_id=%d, biz_token=%s]: %v", userID, rec.ID, rec.UpToken, err)
		return err
	}
	log.Printf("企业实名扫脸结果 [user_id=%d, record_id=%d, result_code=%s, pending=%t]", userID, rec.ID, result.Code, result.Pending)
	now := time.Now()
	if result.Success {
		// 法人扫脸通过：落地企业实名
		if err := s.kybRepo.UpdateResult(rec.ID, 2, "0", "认证成功", "", &now); err != nil {
			return err
		}
		s.applyRealnameStatus(rec.UserID, model.RealnameEnterprise, rec.CompanyName, rec.CreditCode)
		rec.Status, rec.ResultCode, rec.ResultMessage = 2, "0", "认证成功"
		audit.KybRecord("kyb_land", "sync", rec, audit.KV("up_token", rec.UpToken))
		return nil
	}
	// 返回终态失败时（Description 非空）落失败；否则视为仍在认证中，保持待法人扫脸。
	// 失败不写 verified_at（该列语义为认证通过时间，个人实名失败同样不写）
	if !result.Pending && result.Message != "" {
		_ = s.kybRepo.UpdateResult(rec.ID, 3, result.Code, result.Message, "", nil)
		rec.Status, rec.ResultCode, rec.ResultMessage = 3, result.Code, result.Message
		audit.KybRecord("kyb_land", "sync", rec, audit.KV("result_message", result.Message))
	}
	return nil
}

// AdminCreateKybManual 后台人工企业实名（=公户验证）：
// 管理员录入企业名称与统一社会信用代码，直接为企业开通企业实名。
// 开通即通过：写入 verified_at（等于开通时刻，与创建时间同一时刻），供「开通时间」直接取用。
func (s *AuthService) AdminCreateKybManual(userID int64, companyName, creditCode string, adminID int64) error {
	bizNo := generateRecordBizNo(userID) // 企业内部记录单号（userID+毫秒时间戳）
	now := time.Now()
	rec := &model.KybEnterprise{
		UserID:      userID,
		BizNo:       bizNo,
		CompanyName: companyName,
		CreditCode:  creditCode,
		Source:      1, // 后台人工
		AdminID:     adminID,
		Status:      2, // 已通过
		VerifiedAt:  &now,
		CreatedAt:   now, // 与 verified_at 同一时刻：后台人工开通无「核验耗时」
		UpdatedAt:   now,
	}
	if err := s.kybRepo.Create(rec); err != nil {
		return err
	}
	s.applyRealnameStatus(userID, model.RealnameEnterprise, companyName, creditCode)
	audit.KybRecord("kyb_create", "admin_manual", rec, audit.KV("admin_id", adminID))
	return nil
}

// ListKybRecords 企业实名记录列表（管理后台）
func (s *AuthService) ListKybRecords(page, pageSize int) ([]*model.KybEnterprise, int64, error) {
	return s.kybRepo.GetKybRecords(page, pageSize)
}

// ListKycPersonalRecords 后台个人实名记录列表（分页，status=-1 全部）
func (s *AuthService) ListKycPersonalRecords(status, page, pageSize int) ([]*repository.AdminKycPersonal, int64, error) {
	return s.kycRecordRepo.ListAllKycRecords(status, page, pageSize)
}

// GetRecordBestImg 获取认证记录的活体最佳图（下游 API 调用）。
// 约束：认证成功（status=2）后 24 小时内可领取，且每笔订单仅可领取一次。
// 取图顺序：优先返回认证成功时已落盘的照片（零上游调用），
// 本地无文件时才向上游平台按业务号领取活体图（base64）。
func (s *AuthService) GetRecordBestImg(userID int64, bizNo string) (string, error) {
	record, err := s.recordRepo.GetRecordByBizNo(bizNo)
	if err != nil {
		return "", fmt.Errorf("订单不存在")
	}
	if record.UserID != userID {
		return "", fmt.Errorf("订单不存在")
	}
	if record.Status != 2 {
		return "", fmt.Errorf("认证未成功，无法获取活体图片")
	}
	if record.FinishedAt == nil || time.Since(*record.FinishedAt) > 24*time.Hour {
		return "", fmt.Errorf("认证完成已超过24小时，无法获取活体图片")
	}
	if record.BestImgFetched == 1 {
		return "", fmt.Errorf("活体图片仅可领取一次，该订单已领取")
	}
	if record.UpBizID == "" {
		return "", fmt.Errorf("订单缺少上游业务编号")
	}

	// 优先使用认证成功时已保存到本地的活体照片（零上游查询消耗）
	if img, ok := s.readLocalBestImg(record); ok {
		if err := s.recordRepo.UpdateBestImgFetched(record.ID); err != nil {
			log.Printf("标记活体图片已领取失败 [record_id=%d]: %v", record.ID, err)
			return "", fmt.Errorf("领取失败，请稍后重试")
		}
		audit.AuthRecord("record_best_img_fetched", "best_img", record, audit.KV("from", "local"))
		return img, nil
	}

	// 本地无文件时向上游平台领取活体最佳图（base64）
	img, err := s.finAuth.GetBestImg(record.UpBizID)
	if err != nil {
		log.Printf("获取活体最佳图失败 [record_id=%d, biz_id=%s]: %v", record.ID, record.UpBizID, err)
		return "", fmt.Errorf("获取活体图片失败，请稍后重试")
	}
	if img == "" {
		return "", fmt.Errorf("上游未返回活体图片")
	}

	// 取图成功后才落「已领取」标记（避免取图失败占用领取机会）
	if err := s.recordRepo.UpdateBestImgFetched(record.ID); err != nil {
		log.Printf("标记活体图片已领取失败 [record_id=%d]: %v", record.ID, err)
		return "", fmt.Errorf("领取失败，请稍后重试")
	}
	audit.AuthRecord("record_best_img_fetched", "best_img", record, audit.KV("from", "upstream"))
	return img, nil
}

// readLocalBestImg 读取认证成功时已落盘的活体最佳图（base64）；本地无文件或媒体已过期时返回 false。
// 用于避免为取图再发起一次上游调用。
func (s *AuthService) readLocalBestImg(record *model.AuthRecord) (string, bool) {
	if record.MediaDir == "" {
		return "", false
	}
	if record.MediaExpireAt != nil && time.Now().After(*record.MediaExpireAt) {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(s.mediaDir, record.MediaDir, "image_best.jpg"))
	if err != nil || len(data) == 0 {
		return "", false
	}
	return base64.StdEncoding.EncodeToString(data), true
}

// saveRecordMedia 认证成功后自动下载保存照片与视频（保存 30 天，期间可经 /v1/fv/media 下载）。
// 媒体不在结果通知中，需按业务号向上游平台单独领取（照片 jpg 与视频 mp4 均为 base64）。
// 幂等：订单已保存过媒体（media_dir 非空）则跳过。媒体保存失败仅记录日志，不阻断结果落库与下游通知。
func (s *AuthService) saveRecordMedia(record *model.AuthRecord) {
	if record.MediaDir != "" || record.UpBizID == "" {
		return
	}

	media, err := s.finAuth.GetMedia(record.UpBizID)
	if err != nil {
		log.Printf("获取认证媒体失败 [record_id=%d, biz_no=%s]: %v", record.ID, record.BizNo, err)
		return
	}

	relDir := fmt.Sprintf("fv/%s/%s", time.Now().Format("20060102"), record.BizNo)
	dir := filepath.Join(s.mediaDir, relDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("创建认证媒体目录失败 [record_id=%d, dir=%s]: %v", record.ID, dir, err)
		return
	}

	saved := false
	// 保存最佳人脸照片（上游 base64）
	if img := media["image_best"]; img != "" {
		if data, err := base64.StdEncoding.DecodeString(img); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "image_best.jpg"), data, 0o644); err != nil {
				log.Printf("保存认证照片失败 [record_id=%d]: %v", record.ID, err)
			} else {
				saved = true
			}
		} else {
			log.Printf("认证照片 base64 解码失败 [record_id=%d]: %v", record.ID, err)
		}
	}

	// 保存验证视频（上游 base64）
	if video := media["video"]; video != "" {
		if data, err := base64.StdEncoding.DecodeString(video); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "video.mp4"), data, 0o644); err != nil {
				log.Printf("保存认证视频失败 [record_id=%d]: %v", record.ID, err)
			} else {
				saved = true
			}
		} else {
			log.Printf("认证视频 base64 解码失败 [record_id=%d]: %v", record.ID, err)
		}
	}

	if !saved {
		_ = os.RemoveAll(dir)
		return
	}

	expireAt := time.Now().Add(30 * 24 * time.Hour)
	if err := s.recordRepo.UpdateRecordMedia(record.ID, relDir, &expireAt); err != nil {
		log.Printf("记录认证媒体目录失败 [record_id=%d]: %v", record.ID, err)
	}
	log.Printf("认证媒体已保存 [record_id=%d, biz_no=%s, dir=%s, expire_at=%s]", record.ID, record.BizNo, relDir, expireAt.Format("2006-01-02 15:04:05"))
}

// GetRecordMedia 获取认证记录已保存的照片/视频（base64），供下游 /v1/fv/media 调用。
func (s *AuthService) GetRecordMedia(userID int64, bizNo string) (map[string]string, error) {
	record, err := s.recordRepo.GetRecordByBizNo(bizNo)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if record.UserID != userID {
		return nil, fmt.Errorf("订单不存在")
	}
	if record.Status != 2 {
		return nil, fmt.Errorf("认证未成功，无法获取媒体文件")
	}
	if record.MediaDir == "" {
		return nil, fmt.Errorf("媒体文件不存在")
	}
	if record.MediaExpireAt == nil || time.Now().After(*record.MediaExpireAt) {
		return nil, fmt.Errorf("媒体文件已过期")
	}

	dir := filepath.Join(s.mediaDir, record.MediaDir)
	result := make(map[string]string)
	if data, err := os.ReadFile(filepath.Join(dir, "image_best.jpg")); err == nil {
		result["image_best"] = base64.StdEncoding.EncodeToString(data)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "video.mp4")); err == nil {
		result["video"] = base64.StdEncoding.EncodeToString(data)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("媒体文件不存在")
	}
	return result, nil
}

// CleanExpiredMedia 清理已过期的认证媒体文件并清空订单媒体记录（每日定时任务）
func (s *AuthService) CleanExpiredMedia() {
	records, err := s.recordRepo.GetExpiredMediaRecords(time.Now())
	if err != nil {
		log.Printf("查询过期认证媒体订单失败: %v", err)
		return
	}
	for _, o := range records {
		if o.MediaDir == "" {
			continue
		}
		dir := filepath.Join(s.mediaDir, o.MediaDir)
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("删除过期认证媒体失败 [record_id=%d, dir=%s]: %v", o.ID, dir, err)
			continue
		}
		if err := s.recordRepo.ClearRecordMedia(o.ID); err != nil {
			log.Printf("清空过期认证媒体记录失败 [record_id=%d]: %v", o.ID, err)
			continue
		}
		log.Printf("已清理过期认证媒体 [record_id=%d, biz_no=%s]", o.ID, o.BizNo)
	}
}

// errUpQueryExhausted 本单上游可查询次数已用尽。
// 上游 get_result 对同一 biz_id 仅允许 3 次调用（model.UpstreamQueryLimit），
// 用尽后不再请求上游，保持「认证中」，交由上游异步回调或核身超时兜底任务终结，
// 绝不把「查不到结果」当成「认证失败」落地。
var errUpQueryExhausted = errors.New("upstream query quota exhausted")

// queryUpstreamResult 按额度预算调用上游 get_result。
// 先原子递增本单已查询次数，超出 model.UpstreamQueryLimit 时直接返回 errUpQueryExhausted，
// 从源头杜绝「第 4 次调用」——上游此时返回 403 DATA_DESTROYED，该笔结果数据永久不可取回。
// source 记录本次拉取的触发来源（return-核身回跳 poll-承接页/下游轮询 cron-定时任务 manual-后台手动
// best_img-领取活体图），写入审计日志，便于日后复盘「谁在什么时候消耗了额度」。
func (s *AuthService) queryUpstreamResult(record *model.AuthRecord, source string) (*upstream.GetResultResponse, error) {
	if record.UpQueryCount >= model.UpstreamQueryLimit {
		log.Printf("上游结果查询次数已用尽，跳过查询 [record_id=%d, biz_no=%s, up_query_count=%d/%d]",
			record.ID, record.BizNo, record.UpQueryCount, model.UpstreamQueryLimit)
		return nil, errUpQueryExhausted
	}

	count, err := s.recordRepoOf(record).IncrRecordUpQueryCount(record.ID)
	if err != nil {
		return nil, fmt.Errorf("记录上游查询次数失败: %w", err)
	}
	record.UpQueryCount = count
	if count > model.UpstreamQueryLimit {
		// 并发场景下多个请求可能同时通过上面的检查，以数据库计数为准收敛
		log.Printf("上游结果查询次数超限，跳过查询 [record_id=%d, biz_no=%s, up_query_count=%d/%d]",
			record.ID, record.BizNo, count, model.UpstreamQueryLimit)
		return nil, errUpQueryExhausted
	}

	auditUpstreamQuery(record, source)

	return s.finAuth.GetResult(&upstream.GetResultRequest{BizID: record.UpBizID})
}

// auditUpstreamQuery 记录一次「平台主动向上游反查结果」的审计行（含触发来源与已用次数）
func auditUpstreamQuery(record *model.AuthRecord, source string) {
	audit.AuthRecord("upstream_query", source, record,
		audit.KV("quota", fmt.Sprintf("%d/%d", record.UpQueryCount, model.UpstreamQueryLimit)))
}

// upQueryMinInterval 同一订单两次向上游查询之间的最小间隔（仅用于「无回调、只能靠轮询」的订单）。
// 上游每单仅 3 次额度且第 4 次即销毁数据，因此轮询代查必须同时受「次数上限」与「间隔」双重约束：
// 早期实现只有 5 秒节流而无次数上限，下游按 3 秒轮询时几十秒内就把额度打满，
// 用户还没核身完成结果就被销毁，订单被误判失败（历史故障根因）。
const upQueryMinInterval = 60 * time.Second

// syncResultOnPoll 轮询场景下的结果同步（承接页 /service/fv/status、下游 /v1/fv/result、定时任务）。
//
// 仅在「订单没有 notify_url」时代查上游：这类订单平台不会推送、上游也不回传到下游，
// 结果只能靠拉取（如 zjmf 插件「跳过平台异步通知」模式、控制台发起且无回调的订单）；
// 配置了 notify_url 的订单结果由回调落地，轮询一律不消耗额度。
//
// 约束（双重）：① 次数不超过 model.UpstreamQueryLimit；② 第 n 次查询最早发生在
// created_at + n*upQueryMinInterval（用已查次数推导时间门槛，无需额外字段）。
// force=true 表示「用户明确表示已完成核身」（承接页按钮），可跳过最小间隔立即校对一次；
// 次数上限始终生效，绝不产生第 4 次查询。
func (s *AuthService) syncResultOnPoll(record *model.AuthRecord, source string, force bool) {
	if isTerminalStatus(record.Status) || record.UpBizID == "" {
		return
	}
	if record.NotifyURL != "" {
		return
	}
	if record.UpQueryCount >= model.UpstreamQueryLimit {
		return
	}
	if !force && time.Since(record.CreatedAt) < time.Duration(record.UpQueryCount+1)*upQueryMinInterval {
		return
	}
	_ = s.syncRecordResult(record, source, false)
}

// GetRecordStatus 按业务流水号查询认证记录状态（下游 API / 插件轮询）。
//
// 默认不主动请求上游：上游 get_result 对同一 biz_id 仅允许 3 次调用，第 4 次返回 403
// DATA_DESTROYED（结果永久不可取回），而下游插件按秒级轮询，若每次轮询都代查上游，
// 往往在用户还没完成核身时就把额度耗尽、把结果数据打销毁（历史故障根因）。
// 结果落地由三条链路保证：上游异步回调（主力）、用户核身回跳（/v1/fv/return 校对一次）、
// 核身超时兜底任务（超期终结并退款）。
//
// 例外：订单未配置 notify_url（下游「跳过平台异步通知」模式，如 zjmf 插件）时没有任何推送，
// 结果只能靠拉取，此时按「次数上限 + 最小间隔」代查一次（见 syncResultOnPoll）。
// force 表示下游「手动查询/立即校对」（如插件的手动查询按钮），跳过最小间隔立即校对一次；
// 记录已是终态时任何查询都不再请求上游，直接返回本地已落地的结果（见 syncResultOnPoll 首行）
func (s *AuthService) GetRecordStatus(userID int64, bizNo string, force bool) (*model.AuthRecord, error) {
	record, err := s.recordRepo.GetRecordByBizNo(bizNo)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if record.UserID != userID {
		return nil, fmt.Errorf("订单不存在")
	}
	s.syncResultOnPoll(record, "api", force)
	if latest, err := s.recordRepo.GetRecordByID(record.ID); err == nil {
		record = latest
	}
	return record, nil
}

// SyncKycRecord 同步用户最新进行中实名记录的上游结果（账户实名 source=1）
func (s *AuthService) SyncKycRecord(userID int64) (*model.KycPersonal, error) {
	record, err := s.kycRecordRepo.GetPendingByUserID(userID)
	if err != nil {
		return nil, err
	}
	s.syncKycRecordResult(record)
	// 重新查询，获取最新状态
	latest, _ := s.kycRecordRepo.GetLatestByUserID(userID)
	return latest, nil
}

// syncKycRecordResult 同步实名记录结果（从腾讯云人脸核身查询，账户实名 source=1）：
// 成功（ErrCode==0）落地 user 表实名；返回终态失败（Description 非空）落失败；未完成保持认证中。
func (s *AuthService) syncKycRecordResult(record *model.KycPersonal) {
	if s.faceProvider == nil || record.UpToken == "" {
		return
	}

	result, err := s.faceProvider.QueryResult(record.UpToken)
	if err != nil {
		log.Printf("同步实名记录结果失败 [record_id=%d, biz_token=%s]: %v", record.ID, record.UpToken, err)
		return
	}

	now := time.Now()
	if result.Success {
		// 核身通过：落地个人实名
		if err := s.kycRecordRepo.UpdateResult(record.ID, 2, "0", "认证成功", "", &now); err != nil {
			log.Printf("更新实名记录结果失败 [record_id=%d]: %v", record.ID, err)
			return
		}
		s.applyRealnameStatus(record.UserID, model.RealnamePersonal, record.Name, record.IDCard)
		record.Status, record.ResultCode, record.ResultMessage = 2, "0", "认证成功"
		audit.KycRecord("kyc_land", "sync", record)
		return
	}
	// 返回终态失败时（Description 非空）落失败；否则视为仍在认证中，保持认证中状态
	if !result.Pending && result.Message != "" {
		_ = s.kycRecordRepo.UpdateResult(record.ID, 3, result.Code, result.Message, "", nil)
		record.Status, record.ResultCode, record.ResultMessage = 3, result.Code, result.Message
		audit.KycRecord("kyc_land", "sync", record, audit.KV("result_message", result.Message))
	}
}

// CancelKycRecord 取消用户的最新认证记录
func (s *AuthService) CancelKycRecord(userID int64) error {
	kycRecord, err := s.kycRecordRepo.GetLatestByUserID(userID)
	if err != nil {
		return err
	}
	if kycRecord.Status != 1 {
		return fmt.Errorf("no pending kyc record")
	}
	if err := s.kycRecordRepo.Cancel(kycRecord.ID); err != nil {
		return err
	}
	kycRecord.Status = 4
	audit.KycRecord("kyc_cancel", "console", kycRecord)
	return nil
}

// GetUserAuthRecords 查询用户的 FV 认证记录（人脸核验订单 oem_fv.auth_record，
// 账户实名记录属账户级，见实名认证相关接口；可选按创建时间区间过滤）
func (s *AuthService) GetUserAuthRecords(userID int64, page, pageSize int, startDate, endDate string) ([]*model.AuthRecord, int64, error) {
	return s.recordRepo.GetUserAuthRecordsByDate(userID, page, pageSize, startDate, endDate)
}

// GetUserAuthCallStats 统计用户近 N 天的认证调用次数（按天，缺失日期补零）
func (s *AuthService) GetUserAuthCallStats(userID int64, days int) ([]string, []int64, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -(days - 1))

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	stats, err := s.kycRecordRepo.GetUserDailyAuthCount(userID, startStr, endStr)
	if err != nil {
		return nil, nil, err
	}

	dates := make([]string, 0, days)
	counts := make([]int64, 0, days)
	for i := 0; i < days; i++ {
		d := startDate.AddDate(0, 0, i).Format("2006-01-02")
		dates = append(dates, d)
		counts = append(counts, stats[d])
	}

	return dates, counts, nil
}

// applyRealnameStatus 实名成功后落地 user 表实名信息（仅成功后更新）
// status 取 model.RealnamePersonal（个人实名 kyc）/ model.RealnameEnterprise（企业实名 kyb）。
// 更换规则：已企业实名（realname_status=2）后不得降级为个人实名，企业实名结果保持不变；
// 未实名（0）或已个人实名（1）可升级为企业实名。
func (s *AuthService) applyRealnameStatus(userID int64, status int, name, number string) {
	if status == model.RealnamePersonal {
		current, err := s.userRepo.GetUserByID(userID)
		if err != nil {
			log.Printf("查询用户失败，跳过个人实名落地 [user_id=%d]: %v", userID, err)
			return
		}
		// 已企业实名：不允许降级为个人实名，忽略本次个人实名结果
		if current.RealnameStatus == model.RealnameEnterprise {
			log.Printf("用户已企业实名，忽略个人实名落地 [user_id=%d]", userID)
			return
		}
	}
	if err := s.userRepo.UpdateUserRealnameInfo(userID, status, name, number); err != nil {
		log.Printf("更新用户实名信息失败 [user_id=%d]: %v", userID, err)
		return
	}
	audit.Log("user_realname_update",
		audit.KV("user_id", userID),
		audit.KV("realname_status", status))
}

// isTerminalStatus 判断订单是否已处于终态（成功/失败/已取消/超时结束/发起失败）。
// 终态订单不再向上游同步、不接受结果改写，避免重复退款、重复通知与刷新完成时间。
func isTerminalStatus(status int) bool {
	switch status {
	case model.AuthStatusSuccess, model.AuthStatusFailed, model.AuthStatusCanceled,
		model.AuthStatusTimeout, model.AuthStatusStartFail:
		return true
	}
	return false
}

// isInProgressMessage 判断上游返回的 result_message 是否表示「认证尚未开始/进行中」。
// 该状态下认证流程尚未完结，不应视为失败。
func isInProgressMessage(msg string) bool {
	m := strings.ToUpper(strings.TrimSpace(msg))
	switch m {
	case "NOT_STARTED", "PROCESSING", "NOT_START", "IN_PROGRESS", "PENDING":
		return true
	}
	return false
}

// syncRecordResult 同步订单结果（从上游查询，受每单 3 次查询额度约束）。
// 仅用于「结果大概率已就绪」的链路：用户核身回跳校对、核身超期后的一次补查、后台手动查询。
// force=true 表示后台「查询结果」手动核对：终态记录也向上游查询，并按上游结果改写记录；
// force=false 时终态记录直接跳过（避免刷新 finished_at、重复退款与重复通知下游）。
// 返回本次同步的结论文案（后台「查询结果」用于提示管理员），轮询与定时任务调用时忽略该返回值。
// source 为触发来源（return/poll/cron/manual），随审计日志落盘。
func (s *AuthService) syncRecordResult(record *model.AuthRecord, source string, force bool) string {
	if record.UpBizID == "" {
		return "该记录没有上游业务凭证，无法查询"
	}
	if !force && isTerminalStatus(record.Status) {
		return "该记录已是终态，无需查询"
	}

	result, err := s.queryUpstreamResult(record, source)
	if err != nil {
		// 额度用尽：保持「认证中」，等上游回调或超时兜底任务终结，不落地失败
		if errors.Is(err, errUpQueryExhausted) {
			return fmt.Sprintf("上游查询次数已用尽（%d/%d），无法再次查询", record.UpQueryCount, model.UpstreamQueryLimit)
		}
		log.Printf("同步订单结果失败 [biz_id=%s, up_query_count=%d]: %v", record.UpBizID, record.UpQueryCount, err)
		// 上游结果已不可取回（超出可查询次数或超过 1 天有效期）：该笔核身不可能再出结果。
		// 数据销毁只代表「结果无法取回」，不代表核身失败，因此落「超时结束（未完成核身，不计费已退款）」，
		// 并终结订单、退还已扣费用，避免订单永远停在「认证中」被下游无限轮询，也不给下游发假的失败通知。
		if errors.Is(err, upstream.ErrFinAuthDataDestroyed) {
			if isTerminalStatus(record.Status) {
				return "上游结果已不可取回（可查询次数或有效期已用尽），记录已是终态，未改写"
			}
			s.finalizeNotChargeable(record, source, model.AuthStatusTimeout, model.ResultCodeDataDestroyed,
				"上游核身结果已不可取回（可查询次数或有效期已用尽），未完成核身", "认证数据销毁退款")
			return "上游结果已不可取回（可查询次数或有效期已用尽），记录已终结（未完成核身）并退款"
		}
		return fmt.Sprintf("查询上游失败：%v", err)
	}

	// 未开始/进行中：认证尚未完结，保持「认证中」，不结束流程
	if isInProgressMessage(result.ResultMessage) {
		log.Printf("认证尚未开始或进行中，保持认证中状态 [biz_id=%s, result_message=%s]", record.UpBizID, result.ResultMessage)
		return "上游尚未出结果，仍为认证中"
	}

	// 上游未给出有效结果码（0）：不做任何改写，保持原状。
	// 否则会把 result_code 写成「0」、清空 result_message，污染订单结果展示。
	if result.ResultCode == 0 {
		log.Printf("上游未返回有效结果码，保持原状态 [biz_id=%s, result_message=%s]", record.UpBizID, result.ResultMessage)
		return "上游未返回有效结果码，保持原状态"
	}

	// 判断结果状态
	status := record.Status
	switch result.ResultCode {
	case 1000:
		status = 2 // 认证成功
	case 2000, 3000, 4000:
		status = 3 // 认证失败（计费）
	case 6000, 6100:
		status = 3 // 认证失败（不计费）
	default:
		if result.ResultCode != 0 {
			status = 3
		}
	}

	resultCode := fmt.Sprintf("%d", result.ResultCode)
	statusText := "认证失败"
	if status == model.AuthStatusSuccess {
		statusText = "认证成功"
	}

	// 上游结果与本地完全一致：不改写记录（避免无意义写入与重复退款/通知下游）
	if status == record.Status && resultCode == record.ResultCode {
		return fmt.Sprintf("上游结果与本地一致（%s，结果码 %s）", statusText, resultCode)
	}

	if force {
		// 后台手动核对：终态记录也按上游结果改写（不改写 finished_at，避免顺延活体最佳图领取窗口）
		err = s.recordRepoOf(record).UpdateRecordResultForce(record.ID, resultCode, result.ResultMessage, status)
	} else {
		err = s.recordRepoOf(record).UpdateRecordResult(record.ID, resultCode, result.ResultMessage, status)
	}
	if err != nil {
		log.Printf("更新订单结果失败 [record_id=%d]: %v", record.ID, err)
		return fmt.Sprintf("更新记录结果失败：%v", err)
	}

	// 不计费结果（6000/6100）退还预扣余额
	s.refundIfNotChargeable(record, source, int(result.ResultCode))

	// 认证记录（下游 API 调用）结果只落在 oem_fv.auth_record；
	// 账户实名记录（kyc）由 syncKycRecordResult 依据腾讯云人脸核身结果单独落地，不回写
	if status == model.AuthStatusSuccess {
		// 认证成功后自动下载保存照片与视频（30 天，供下游 API 下载）
		s.saveRecordMedia(record)
	}

	// 通知下游
	record.Status = status
	record.ResultCode = resultCode
	record.ResultMessage = result.ResultMessage
	s.NotifyDownstream(record)

	audit.AuthRecord("record_land", source, record,
		audit.KV("result_message", result.ResultMessage),
		audit.KV("force", force))

	return fmt.Sprintf("查询成功：%s（结果码 %s）", statusText, resultCode)
}

// QueryRecordResultForAdmin 后台手动查询认证记录的上游结果（调上游核对并回写本地）。
// 强制向上游查询并允许改写终态记录（不改写 finished_at），以最后一次上游结果为准。
func (s *AuthService) QueryRecordResultForAdmin(recordID int64) (*model.AuthRecord, string, error) {
	record, err := s.recordRepo.GetRecordByID(recordID)
	if err != nil {
		return nil, "", fmt.Errorf("认证记录不存在")
	}

	msg := s.syncRecordResult(record, "manual", true)

	latest, err := s.recordRepo.GetRecordByID(recordID)
	if err != nil {
		return record, msg, nil
	}
	return latest, msg, nil
}

// HandleUpstreamCallback 处理上游异步回调
// data: JSON 字符串，sign: HMAC 签名
func (s *AuthService) HandleUpstreamCallback(data, sign string) error {
	// 1. 验证签名（FinAuth 下游凭据）
	if !s.finAuth.VerifySign(data, sign) {
		log.Printf("回调签名验证失败: data=%s, sign=%s", data, sign)
		logstore.RecordSysCall("callback", "finauth", "", 0, "", upstream.RedactPayload([]byte(data)),
			"signature verification failed", 0)
		return errors.New("signature verification failed")
	}

	// 2. 解析回调数据
	var notifyData upstream.NotifyData
	if err := json.Unmarshal([]byte(data), &notifyData); err != nil {
		log.Printf("解析回调数据失败: %v, data=%s", err, data)
		logstore.RecordSysCall("callback", "finauth", "", 0, "", upstream.RedactPayload([]byte(data)),
			fmt.Sprintf("parse failed: %v", err), 0)
		return fmt.Errorf("parse notify data failed: %w", err)
	}

	// 2.1 记录回调原文（脱敏）到 syscall.log：biz_no/biz_id 可 join，result_code 即上游结论，
	// 这是「结果由回调落地」这条链路唯一的复现依据（此前只记了方法/路径/状态码，无法复盘）
	logstore.RecordSysCall("callback", "finauth", notifyData.BizInfo.BizNo, 0, notifyData.BizInfo.BizID,
		upstream.RedactPayload([]byte(data)),
		fmt.Sprintf("result_code=%d result_message=%s", notifyData.ResultCode, notifyData.ResultMessage), 1)

	// 3. 根据 biz_id 查找认证记录（人脸核验订单库）
	record, err := s.recordRepo.GetRecordByUpBizID(notifyData.BizInfo.BizID)
	if err != nil {
		log.Printf("查找认证记录失败 [biz_id=%s]: %v", notifyData.BizInfo.BizID, err)
		return fmt.Errorf("record not found: %w", err)
	}

	// 未开始/进行中：认证尚未完结，忽略本次回调，保持「认证中」
	if isInProgressMessage(notifyData.ResultMessage) {
		log.Printf("回调表示认证尚未开始或进行中，忽略 [biz_id=%s, result_message=%s]", notifyData.BizInfo.BizID, notifyData.ResultMessage)
		return nil
	}

	// 终态订单忽略回调重放：不降级状态、不刷新 finished_at、不重复退款/通知
	if isTerminalStatus(record.Status) {
		log.Printf("订单已是终态，忽略回调 [biz_id=%s, status=%d, result_code=%d]", notifyData.BizInfo.BizID, record.Status, notifyData.ResultCode)
		return nil
	}

	// 4. 判断认证结果
	status := record.Status
	switch notifyData.ResultCode {
	case 1000:
		status = 2 // 认证成功
	case 2000, 3000, 4000:
		status = 3 // 认证失败（计费）
	case 6000, 6100:
		status = 3 // 认证失败（不计费）
	default:
		status = 3
	}

	resultCode := fmt.Sprintf("%d", notifyData.ResultCode)
	err = s.recordRepoOf(record).UpdateRecordResult(record.ID, resultCode, notifyData.ResultMessage, status)
	if err != nil {
		log.Printf("更新订单结果失败 [record_id=%d]: %v", record.ID, err)
		return fmt.Errorf("update record result failed: %w", err)
	}

	// 不计费结果（6000/6100）退还预扣余额
	s.refundIfNotChargeable(record, "callback", int(notifyData.ResultCode))

	// 认证记录（下游 API 调用）结果只落在 oem_fv.auth_record；
	// 账户实名记录（kyc）由 syncKycRecordResult 依据腾讯云人脸核身结果单独落地，不回写
	if status == 2 {
		// 认证成功后自动下载保存照片与视频（30 天，供下游 API 下载）
		s.saveRecordMedia(record)
	}

	// 通知下游
	record.Status = status
	record.ResultCode = resultCode
	record.ResultMessage = notifyData.ResultMessage
	s.NotifyDownstream(record)

	audit.AuthRecord("record_land", "callback", record, audit.KV("result_message", notifyData.ResultMessage))

	log.Printf("回调处理成功 [biz_id=%s, result_code=%d]", notifyData.BizInfo.BizID, notifyData.ResultCode)
	return nil
}

// NotifyDownstream 通知下游：将认证结果 POST 到下游的 notify_url（携带 HMAC 签名，供下游校验防伪造）
func (s *AuthService) NotifyDownstream(record *model.AuthRecord) {
	if record.NotifyURL == "" {
		return
	}

	// 使用发起该订单的 API 密钥 secret 生成签名（下游用同一 secret 校验）；
	// 历史订单未记录发起密钥（api_id=0）时回退到该账号主密钥
	sign := ""
	if cred := resolveRecordAPIKey(s.apiKeyRepo, record.APIID, record.UserID); cred != nil && cred.APISecret != "" {
		sign = buildNotifySign(cred.APISecret, record)
	}

	payload := map[string]interface{}{
		"biz_no":         record.BizNo,
		"status":         record.Status,
		"result_code":    record.ResultCode,
		"result_message": record.ResultMessage,
		"cost":           record.Cost,
		"sign":           sign,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("序列化通知数据失败 [record_id=%d]: %v", record.ID, err)
		return
	}

	resp, err := http.Post(record.NotifyURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("通知下游失败 [record_id=%d, url=%s]: %v", record.ID, record.NotifyURL, err)
		s.notifySvc.Enqueue("fv_result", record.ID, record.UserID, record.NotifyURL, string(jsonData))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("通知下游成功 [record_id=%d, url=%s, status=%d]", record.ID, record.NotifyURL, resp.StatusCode)
	} else {
		log.Printf("通知下游返回异常 [record_id=%d, url=%s, status=%d]", record.ID, record.NotifyURL, resp.StatusCode)
		s.notifySvc.Enqueue("fv_result", record.ID, record.UserID, record.NotifyURL, string(jsonData))
	}
}

// resolveRecordAPIKey 解析用于下游回调签名的 API 密钥：
// 优先取发起该业务的密钥（apiID），未记录（历史数据/控制台发起）时回退到该账号主密钥（最早创建的一把）。
func resolveRecordAPIKey(repo *repository.ApiKeyRepository, apiID, userID int64) *model.ApiKey {
	if apiID > 0 {
		if k, err := repo.GetByID(apiID); err == nil && k != nil {
			return k
		}
	}
	if k, err := repo.GetByUser(userID); err == nil && k != nil {
		return k
	}
	return nil
}

// buildNotifySign 构造下游回调签名：
// 对固定字段按 key 字典序拼接为 k=v&k=v... 的原始字符串（不做 URL 编码），
// 再以 HMAC-SHA256(api_secret, canonical) 计算十六进制小写签名。
// 下游（如 zjmf_v10 插件）用相同算法与自己的 api_secret 校验，杜绝伪造回调。
func buildNotifySign(apiSecret string, record *model.AuthRecord) string {
	fields := map[string]string{
		"biz_no":         record.BizNo,
		"cost":           strconv.FormatFloat(record.Cost, 'f', 2, 64),
		"result_code":    record.ResultCode,
		"result_message": record.ResultMessage,
		"status":         strconv.Itoa(record.Status),
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

// markRecordStartFailed 发起认证被上游拦截/调用失败时：扣费发生在上游接受核验之后，此处尚未扣费，
// 故不涉及退款；保留订单并写入失败原因（status=6 发起失败），便于排查与对账。
func (s *AuthService) markRecordStartFailed(record *model.AuthRecord) {
	if err := s.recordRepoOf(record).UpdateRecordResult(record.ID, model.ResultCodeTimeout, "连接超时", model.AuthStatusStartFail); err != nil {
		log.Printf("更新订单连接超时失败 [record_id=%d]: %v", record.ID, err)
	}
	record.Status, record.ResultCode, record.ResultMessage = model.AuthStatusStartFail, model.ResultCodeTimeout, "连接超时"
	audit.AuthRecord("record_start_failed", "api", record)
}

// refundChargedRecord 退还已扣费用（资源包次数/余额）并标记已退款。
// 未实际扣费或已退款时直接返回 false（幂等），退款成功后同步内存中的退款标记。
func (s *AuthService) refundChargedRecord(record *model.AuthRecord, source, remark string) bool {
	if !hasCharged(record) || record.IsRefunded == 1 {
		return false
	}
	if err := s.refundRecordCharge(record, remark); err != nil {
		log.Printf("%s失败 [record_id=%d]: %v", remark, record.ID, err)
		return false
	}
	if err := s.recordRepoOf(record).UpdateRecordRefundFlag(record.ID); err != nil {
		log.Printf("更新退款标记失败 [record_id=%d]: %v", record.ID, err)
		return false
	}
	record.IsRefunded = 1
	audit.AuthRecord("record_refund", source, record, audit.KV("remark", remark))
	return true
}

// finalizeNotChargeable 将订单落到「核身无结果、不计费」终态：写入结果码/结果信息/状态，
// 已扣费用按 remark 原路退还（资源包退次数、余额退余额），并通知下游。
// 用于认证数据销毁、核身超时未完成等场景，保证订单不再停在「认证中」被无限轮询。
// 已处于终态的订单直接跳过（避免重复退款与重复通知）。
func (s *AuthService) finalizeNotChargeable(record *model.AuthRecord, source string, status int, resultCode, resultMessage, refundRemark string) {
	if isTerminalStatus(record.Status) {
		return
	}
	if err := s.recordRepoOf(record).UpdateRecordResult(record.ID, resultCode, resultMessage, status); err != nil {
		log.Printf("更新订单结果失败 [record_id=%d]: %v", record.ID, err)
		return
	}
	s.refundChargedRecord(record, source, refundRemark)

	record.Status = status
	record.ResultCode = resultCode
	record.ResultMessage = resultMessage
	s.NotifyDownstream(record)

	audit.AuthRecord("record_finalize", source, record, audit.KV("result_message", resultMessage))
}

// defaultAuthExpireMinutes 上游未返回 token 到期时间时，核身链接的兜底有效期（分钟），
// 与对外返回的链接有效期口径一致（见 authExpiredAt）。
const defaultAuthExpireMinutes = 15

// ExpirePendingRecords 终结核身超时未完成的订单：以 token 到期时间为准（缺省按创建时间 + 15 分钟），
// 超期仍未出结果即视为未完成核身，置「超时结束」并退还已扣费用（资源包次数/余额），同时通知下游。
// 兜底上游既不返回结果也不销毁数据的场景（用户拿到链接后始终未核身）。
func (s *AuthService) ExpirePendingRecords() error {
	records, err := s.recordRepo.GetExpiredPendingRecords(defaultAuthExpireMinutes)
	if err != nil {
		return err
	}
	for _, record := range records {
		log.Printf("核身超时未完成，终结订单并退款 [record_id=%d, biz_no=%s]", record.ID, record.BizNo)
		s.finalizeNotChargeable(record, "cron", model.AuthStatusTimeout, model.ResultCodeExpired,
			"核身链接超时未完成", "核身超时退款")
	}
	return nil
}

// refundIfNotChargeable 不计费结果（6000/6100）退还未计费部分的费用（资源包次数/余额）
func (s *AuthService) refundIfNotChargeable(record *model.AuthRecord, source string, resultCode int) {
	if resultCode != 6000 && resultCode != 6100 {
		return
	}
	// 未实际扣费（免费或扣费失败）无需退款；已退款时幂等跳过
	s.refundChargedRecord(record, source, "认证不计费退款")
}

// SyncPendingRecords 同步处理中订单的上游结果（账户实名走腾讯云，人脸核验走 FinAuth）。
func (s *AuthService) SyncPendingRecords() error {
	fvRecords, err := s.recordRepo.GetPendingRecords()
	if err != nil {
		return err
	}

	// 人脸核验：**不做全量上游查询**。上游 get_result 对同一 biz_id 仅允许 3 次调用
	// （第 4 次返回 403 DATA_DESTROYED，结果永久不可取回），定时任务全量轮查会在用户
	// 尚未完成核身时就把额度耗尽并销毁结果数据（历史故障根因）。
	// 这里只在「已过核身有效期、仍无结果」时补查一次（额度允许时）：若上游其实已出结果
	// （回调丢失/用户未回跳）即可正常落地；否则交由超时兜底任务终结并退款。
	for _, record := range fvRecords {
		// 无 notify_url：结果只能靠拉取，按「3 次额度 + 60 秒间隔」阶梯代查
		if record.NotifyURL == "" {
			s.syncResultOnPoll(record, "cron", false)
			continue
		}
		// 有回调但核身有效期已过仍无结果（回调可能丢失/用户未回跳）：补查一次
		if record.TokenExpireAt != nil && !time.Now().Before(*record.TokenExpireAt) {
			_ = s.syncRecordResult(record, "cron", false)
		}
	}

	// 账户实名（source=1）：同步处理中的实名记录（无订单）
	records, err := s.kycRecordRepo.GetPendingRecords()
	if err != nil {
		return err
	}
	for _, record := range records {
		s.syncKycRecordResult(record)
	}
	return nil
}
