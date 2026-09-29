package upstream

// FinAuthInterface 人脸核验（FV）产品上游接口。
// OEM 系统的实现为 StarLoft 平台开放 API 客户端（见 internal/upstream/starloft）。
type FinAuthInterface interface {
	GetToken(req *GetTokenRequest) (*GetTokenResponse, error)
	GetResult(req *GetResultRequest) (*GetResultResponse, error)
	// GetBestImg 领取活体最佳图（base64），成功后每单仅可领取一次
	GetBestImg(bizNo string) (string, error)
	// GetMedia 下载认证照片与视频（base64），键为 image_best / video
	GetMedia(bizNo string) (map[string]string, error)
	VerifySign(jsonData, receivedSign string) bool
}