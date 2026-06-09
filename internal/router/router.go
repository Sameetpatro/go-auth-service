package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	jwtsvc "github.com/sameetpatro/go-qr-auth/internal/auth"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/handler"
	"github.com/sameetpatro/go-qr-auth/internal/middleware"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/websocket"
)

type Handlers struct {
	Auth         *handler.AuthHandler
	Coordinator  *handler.CoordinatorHandler
	Guest        *handler.GuestHandler
	Scan         *handler.ScanHandler
	Analytics    *handler.AnalyticsHandler
	WSHub        *websocket.Hub
	JWT          *jwtsvc.Service
	Config       *config.Config
}

func Setup(h Handlers) *gin.Engine {
	if h.Config.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.CORS(h.Config.CORS.AllowedOrigins))
	r.Use(middleware.RateLimit(h.Config.RateLimit.RequestsPerMinute))

	r.GET("/health", handler.Health)
	r.Static("/storage/qr", h.Config.Storage.QRImagePath)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/ws", func(c *gin.Context) {
		h.WSHub.HandleWebSocket(c.Writer, c.Request)
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", h.Auth.Login)
			auth.POST("/refresh", h.Auth.Refresh)
		}

		protected := api.Group("")
		protected.Use(middleware.JWTAuth(h.JWT))
		{
			protected.POST("/scan", h.Scan.Scan)

			guests := protected.Group("/guests")
			{
				guests.GET("", h.Guest.List)
				guests.GET("/search", h.Guest.Search)
				guests.GET("/verify", h.Guest.VerifySearch)
				guests.POST("", h.Guest.Create)
				guests.POST("/import", h.Guest.Import)
				guests.GET("/:id", h.Guest.Get)

				masterGuests := guests.Group("")
				masterGuests.Use(middleware.RequireRole(models.RoleMaster))
				masterGuests.POST("/invite-all", h.Guest.InviteAll)
				masterGuests.PUT("/:id", h.Guest.Update)
				masterGuests.DELETE("/:id", h.Guest.Delete)
			}

			analytics := protected.Group("/analytics")
			analytics.GET("/dashboard", h.Analytics.Dashboard)

			protected.GET("/insights", h.Analytics.Insights)

			reports := protected.Group("/reports")
			reports.GET("/export/csv", h.Analytics.ExportCSV)
			reports.GET("/export/pdf", h.Analytics.ExportPDF)

			coordinators := protected.Group("/coordinators")
			coordinators.Use(middleware.RequireRole(models.RoleMaster))
			{
				coordinators.POST("", h.Coordinator.Create)
				coordinators.GET("", h.Coordinator.List)
				coordinators.PATCH("/:id/disable", h.Coordinator.Disable)
				coordinators.POST("/:id/reset-password", h.Coordinator.ResetPassword)
			}
		}
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Route not found"})
	})

	return r
}
