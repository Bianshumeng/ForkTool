package routes

type router struct{}

type routeGroup struct{}

type gatewayHandlers struct {
	Gateway       gatewayHandlerSet
	OpenAIGateway openAIGatewayHandlerSet
}

type gatewayHandlerSet struct{}

type openAIGatewayHandlerSet struct{}

func (router) Group(_ string) routeGroup { return routeGroup{} }

func (routeGroup) POST(_ string, _ ...any) {}

func (gatewayHandlerSet) Messages(_ any) {}

func (gatewayHandlerSet) CountTokens(_ any) {}

func (openAIGatewayHandlerSet) Responses(_ any) {}

func RegisterGatewayRoutes(r router, h gatewayHandlers) {
	gateway := r.Group("/v1")
	gateway.POST("/messages", h.Gateway.Messages)
	gateway.POST("/messages/count_tokens", func(c any) {
		h.Gateway.CountTokens(c)
	})
	gateway.POST("/responses/*subpath", func(c any) {
		h.OpenAIGateway.Responses(c)
	})
}
