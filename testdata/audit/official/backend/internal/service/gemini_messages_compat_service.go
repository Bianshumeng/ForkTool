package service

type ForwardResult struct {
	UpstreamModel string
}

func Forward() ForwardResult {
	result := ForwardResult{}
	result.UpstreamModel = "gemini-2.0"
	return result
}
