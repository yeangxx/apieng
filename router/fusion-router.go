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
		keyRoute := fusionRoute.Group("/keys")
		keyRoute.GET("", controller.GetFusionAPIKeys)
		keyRoute.POST("", controller.CreateFusionAPIKey)
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
}
