package upstream

// FinAuthInterface FinAuth 客户端接口
type FinAuthInterface interface {
	GetToken(req *GetTokenRequest) (*GetTokenResponse, error)
	GetResult(req *GetResultRequest) (*GetResultResponse, error)
	VerifySign(jsonData, receivedSign string) bool
	DownloadVideo(videoURL string) ([]byte, error)
}

// 确保实现符合接口
var _ FinAuthInterface = (*FinAuthClient)(nil)
