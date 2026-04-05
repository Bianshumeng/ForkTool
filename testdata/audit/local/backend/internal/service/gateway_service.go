package service

func GenerateSessionHash(userAgent string) string {
	return userAgent
}

func forwardHeaders() {
	HeaderAdd("x-api-key")
}

func ForwardCountTokens() string {
	return buildCountTokensRequest()
}

func buildCountTokensRequest() string {
	return "/v1/messages/count_tokens"
}

func HeaderAdd(_ string) {}
