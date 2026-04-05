package service

func buildUpstreamRequest() string {
	return "/v1/responses"
}

func passthroughBody() string {
	return "noop"
}
