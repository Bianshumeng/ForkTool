package service

func NormalizeSessionUserAgent(value string) string {
	return value
}

func GenerateGeminiPrefixHash(userAgent string) string {
	return NormalizeSessionUserAgent(userAgent)
}
