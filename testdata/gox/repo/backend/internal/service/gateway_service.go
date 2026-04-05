package service

func ForwardCountTokens() string {
	return buildCountTokensSuffix(buildCountTokensRequest())
}

func buildCountTokensRequest() string {
	return "/v1/messages/count_tokens?beta=true"
}

func buildCountTokensSuffix(path string) string {
	return path
}
