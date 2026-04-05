package handler

import "testdata/gox/repo/backend/internal/service"

type GatewayHandler struct{}

type OpenAIGatewayHandler struct{}

func (GatewayHandler) Messages(_ any) {}

func (GatewayHandler) CountTokens(_ any) {
	service.ForwardCountTokens()
}

func (OpenAIGatewayHandler) Responses(_ any) {}
