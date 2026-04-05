package routes

func RegisterOpenAIRoutes() {
	registerPOST("/v1/responses")
	registerPOST("/v1/responses/compact")
}

func registerPOST(_ string) {}
