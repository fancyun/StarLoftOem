package upstream

import "crypto/aes"

// videoAESKey 生成验证视频 AES-256-ECB 解密密钥：
// key = api_secret 前 32 字节，不足 32 字节时以空格补足。
func videoAESKey(apiSecret string) []byte {
	key := make([]byte, 32)
	for i := range key {
		if i < len(apiSecret) {
			key[i] = apiSecret[i]
		} else {
			key[i] = ' '
		}
	}
	return key
}

// DecryptVideoAESECB 解密上游验证视频（AES-256-ECB）：
// 仅加密前 1024 字节（64 个 16 字节块），其后为明文；长度不足 1024 时仅解密整块部分。
// 解密失败时原样返回，不阻断视频保存流程。
func DecryptVideoAESECB(data []byte, apiSecret string) []byte {
	block, err := aes.NewCipher(videoAESKey(apiSecret))
	if err != nil {
		return data
	}

	encLen := len(data)
	if encLen > 1024 {
		encLen = 1024
	}
	encLen -= encLen % aes.BlockSize
	if encLen <= 0 {
		return data
	}

	dec := make([]byte, encLen)
	for i := 0; i < encLen; i += aes.BlockSize {
		block.Decrypt(dec[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	return append(dec, data[encLen:]...)
}
