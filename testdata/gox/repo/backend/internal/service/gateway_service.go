package service

func ForwardCountTokens() string {
	return buildCountTokensRequest()
}

func buildCountTokensRequest() string {
	return "/v1/messages/count_tokens?beta=true"
}
