package handler

type GatewayHandler struct{}

type OpenAIGatewayHandler struct{}

func (GatewayHandler) Messages(_ any) {}

func (GatewayHandler) CountTokens(_ any) {}

func (OpenAIGatewayHandler) Responses(_ any) {}
