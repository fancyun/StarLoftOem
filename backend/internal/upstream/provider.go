package upstream

// 本文件定义可切换的账户级能力 provider 抽象：账户实名的人脸核身、企业工商四要素核验。
// 各 provider 实现互不依赖，由运行时按后台配置装配，业务层只依赖接口。

// FaceStartResult 人脸核身发起结果（provider 无关）
type FaceStartResult struct {
	AuthURL  string // 供浏览器直接访问的核身地址
	Token    string // 结果查询标识（腾讯：BizToken；StarLoft 平台：业务号）
	ExpireAt int64  // 核身地址有效期（Unix 秒，0 表示未知）
}

// FaceQueryResult 人脸核身结果（provider 无关）
type FaceQueryResult struct {
	Success bool   // 是否核验通过
	Pending bool   // 是否仍在进行中（未出终态）
	Code    string // 上游结果码（原样）
	Message string // 结果描述（终态失败时非空）
}

// FaceProvider 账户实名人脸核身 provider（腾讯云人脸核身 / StarLoft 平台可切换）
type FaceProvider interface {
	StartAuth(name, idCard, returnURL, bizNo string) (*FaceStartResult, error)
	QueryResult(token string) (*FaceQueryResult, error)
}

// EnterpriseVerifyResult 企业工商四要素核验结果（企业名 + 统一社会信用代码 + 法人姓名 + 法人身份证号）
type EnterpriseVerifyResult struct {
	Matched bool   // 四要素是否一致
	Message string // 不通过时的说明
	RawData string // 上游原始返回（JSON 文本，落库备查）
}

// EnterpriseVerifier 企业工商四要素核验 provider（腾讯云 OCR / 阿里云可切换）
type EnterpriseVerifier interface {
	Verify(companyName, creditCode, legalName, legalIDCard string) (*EnterpriseVerifyResult, error)
}

// SmsPushVerifier 上游短信推送（回执/回复/签名与模板状态）的签名校验器：
// 校验失败时业务侧一律丢弃该次推送，防止伪造回调改写发送状态。
type SmsPushVerifier interface {
	VerifySmsReceiptSign(body []byte) bool
	VerifySmsReplySign(body []byte) bool
	VerifySmsStatusSign(body []byte) bool
}