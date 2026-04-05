package routes

func RegisterGatewayRoutes() {
	registerPOST("/v1/messages", "Messages")
	registerPOST("/v1/messages/count_tokens", "CountTokens")
}

func registerPOST(_ string, _ string) {}
