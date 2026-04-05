package service

func buildUpstreamRequest() string {
	return "/v1/responses/compact"
}

func isolateOpenAISessionID(value string) string {
	return value
}

func normalizeOpenAIPassthroughOAuthBody() string {
	return "normalized"
}
