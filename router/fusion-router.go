package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterFusionAPIRoutes(apiRouter *gin.RouterGroup) {
	fusionRoute := apiRouter.Group("/fusion")
	fusionRoute.Use(middleware.UserAuth())
	{
		fusionRoute.GET("/upstream-templates", controller.GetFusionUpstreamTemplates)

		keyRoute := fusionRoute.Group("/keys")
		keyRoute.GET("", controller.GetFusionAPIKeys)
		keyRoute.POST("", controller.CreateFusionAPIKey)
		keyRoute.POST("/test", controller.TestUnsavedFusionAPIKey)
		keyRoute.PUT("/:id", controller.UpdateFusionAPIKey)
		keyRoute.DELETE("/:id", controller.DeleteFusionAPIKey)
		keyRoute.POST("/:id/test", controller.TestFusionAPIKey)

		configRoute := fusionRoute.Group("/configs")
		configRoute.GET("", controller.GetFusionConfigs)
		configRoute.POST("", controller.CreateFusionConfig)
		configRoute.PUT("/:id", controller.UpdateFusionConfig)
		configRoute.DELETE("/:id", controller.DeleteFusionConfig)
		configRoute.POST("/:id/test", controller.TestFusionConfig)
	}

	adminRoute := apiRouter.Group("/fusion/admin")
	adminRoute.Use(middleware.AdminAuth())
	{
		templateRoute := adminRoute.Group("/upstream-templates")
		templateRoute.GET("", controller.AdminGetFusionUpstreamTemplates)
		templateRoute.POST("", controller.AdminCreateFusionUpstreamTemplate)
		templateRoute.PUT("/:id", controller.AdminUpdateFusionUpstreamTemplate)
		templateRoute.DELETE("/:id", controller.AdminDeleteFusionUpstreamTemplate)
	}
}
