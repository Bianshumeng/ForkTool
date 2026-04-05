package service

func GenerateSessionHash(userAgent string) string {
	return NormalizeSessionUserAgent(userAgent)
}

func forwardHeaders() {
	resolveWireCasing("x-api-key")
	addHeaderRaw("x-api-key")
}

func ForwardCountTokens() string {
	return buildCountTokensRequest()
}

func buildCountTokensRequest() string {
	return "/v1/messages/count_tokens?beta=true"
}

func resolveWireCasing(_ string) {}

func addHeaderRaw(_ string) {}
