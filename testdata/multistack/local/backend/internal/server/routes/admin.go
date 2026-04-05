package routes

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(r *gin.Engine, h *Handlers) {
	admin := r.Group("/admin")
	admin.GET("/settings/status-page", h.Settings.StatusPage)
}

type Handlers struct {
	Settings SettingsHandler
}

type SettingsHandler interface {
	StatusPage(*gin.Context)
}
