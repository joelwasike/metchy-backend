package router

import (
	"log"
	"time"

	"lusty/config"
	"lusty/internal/handler"
	"lusty/internal/middleware"
	"lusty/internal/repository"
	"lusty/internal/service"
	"lusty/internal/ws"
	"lusty/pkg/cloudinary"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(cfg *config.Config, db *gorm.DB, cloud cloudinary.Client) *gin.Engine {
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	// Skip gin.Logger() to reduce log noise; use gin.Default() if you need request logging
	r.Use(middleware.RateLimit(middleware.NewInMemoryRateLimiter(100, 60*time.Second)))

	// Repositories
	userRepo := repository.NewUserRepository(db)
	locRepo := repository.NewLocationRepository(db)
	presenceRepo := repository.NewPresenceRepository(db)
	companionRepo := repository.NewCompanionRepository(db)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	favRepo := repository.NewFavoriteRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	blockRepo := repository.NewBlockRepository(db)
	reportRepo := repository.NewReportRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	interactionRepo := repository.NewInteractionRepository(db)
	walletRepo := repository.NewWalletRepository(db)

	mapHub := ws.NewMapHub()
	chatHub := ws.NewChatHub()
	videoHub := ws.NewVideoHub()

	// Services
	authSvc := service.NewAuthService(cfg, userRepo)
	fcmSvc := service.NewFCMService(cfg.Firebase.ServiceAccountPath)
	if fcmSvc != nil {
		log.Printf("[FCM] Push notifications enabled")
	} else if cfg.Firebase.ServiceAccountPath != "" {
		log.Printf("[FCM] Push notifications disabled: failed to init (check service account file)")
	} else {
		log.Printf("[FCM] Push notifications disabled: set FIREBASE_SERVICE_ACCOUNT_PATH to enable")
	}
	notifSvc := service.NewNotificationService(notificationRepo, userRepo, fcmSvc)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, presenceRepo, auditRepo, companionRepo)
	meHandler := handler.NewMeHandler(userRepo, companionRepo, locRepo, favRepo, paymentRepo, interactionRepo, walletRepo)
	googleOAuthHandler := handler.NewGoogleOAuthHandler(cfg, authSvc, presenceRepo, auditRepo)
	discoveryHandler := handler.NewDiscoveryHandler(discoveryRepo)
	companionHandler := handler.NewCompanionHandler(companionRepo, userRepo, interactionRepo, cloud)
	locationHandler := handler.NewLocationHandler(locRepo, presenceRepo, companionRepo, cfg, mapHub)
	presenceHandler := handler.NewPresenceHandler(presenceRepo, companionRepo, favRepo, notifSvc)
	favoriteHandler := handler.NewFavoriteHandler(favRepo, companionRepo)
	blockHandler := handler.NewBlockHandler(blockRepo)
	reportHandler := handler.NewReportHandler(reportRepo, auditRepo)
	notificationHandler := handler.NewNotificationHandler(notificationRepo)
	pricingHandler := handler.NewPricingHandler(companionRepo)
	boostHandler := handler.NewBoostHandler(companionRepo)
	interactionHandler := handler.NewInteractionHandler(interactionRepo, companionRepo, paymentRepo, walletRepo, userRepo, notifSvc)
	paymentWebhookHandler := handler.NewPaymentWebhookHandler(paymentRepo, auditRepo, notifSvc, cfg)
	walletHandler := handler.NewWalletHandler(walletRepo)
	mpesaHandler := handler.NewMpesaHandler(cfg, paymentRepo, interactionRepo, companionRepo, walletRepo, userRepo, notifSvc)
	mpesaWebhookHandler := handler.NewMpesaWebhookHandler(paymentRepo, interactionRepo, companionRepo, walletRepo, auditRepo, notifSvc)
	withdrawalRepo := repository.NewWithdrawalRepository(db)
	withdrawalHandler := handler.NewWithdrawalHandler(cfg, walletRepo, withdrawalRepo, companionRepo)
	withdrawalWebhookHandler := handler.NewWithdrawalWebhookHandler(withdrawalRepo, walletRepo)
	chatHandler := handler.NewChatHandler(interactionRepo, companionRepo)
	uploadHandler := handler.NewUploadHandler(cloud)
	distanceHandler := handler.NewDistanceHandler(interactionRepo, companionRepo, locRepo, userRepo)

	authMw := middleware.AuthRequired(&cfg.JWT)
	adultMw := middleware.AdultOnly(cfg, userRepo)

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/logout", authMw, authHandler.Logout)
			authGroup.PATCH("/change-password", authMw, authHandler.ChangePassword)
			authGroup.POST("/refresh", authHandler.Refresh)
			authGroup.GET("/google", googleOAuthHandler.Redirect)
			authGroup.GET("/google/callback", googleOAuthHandler.Callback)
			authGroup.POST("/google/token", googleOAuthHandler.Token)
		}

		api.GET("/discover", authMw, adultMw, discoveryHandler.Discover)
		api.GET("/companions/:id", authMw, adultMw, companionHandler.GetProfile)

		me := api.Group("/me")
		me.Use(authMw)
		{
			me.GET("/profile", meHandler.GetProfile)
			me.POST("/onboarding/complete", meHandler.CompleteOnboarding) // no adult - may set DOB for Google signups
		}
		meAdult := api.Group("/me")
		meAdult.Use(authMw, adultMw)
		{
			meAdult.PATCH("/settings", meHandler.UpdateSettings)
			meAdult.GET("/dashboard", meHandler.GetDashboard)
			meAdult.PATCH("/location", locationHandler.UpdateLocation)
			meAdult.GET("/location", locationHandler.GetMyLocation)
			meAdult.PATCH("/presence", presenceHandler.SetPresence)
			meAdult.GET("/presence", presenceHandler.GetMyPresence)
			meAdult.GET("/favorites", favoriteHandler.List)
			meAdult.GET("/notifications", notificationHandler.List)
			meAdult.PUT("/notifications/:id/read", notificationHandler.MarkRead)
			meAdult.GET("/wallet", walletHandler.GetBalance)
			meAdult.GET("/wallet/transactions", walletHandler.GetTransactions)
			meAdult.POST("/withdraw", withdrawalHandler.Create)
			meAdult.GET("/active-sessions", meHandler.GetActiveSessions)
			meAdult.GET("/fans", meHandler.GetFans)
			meAdult.GET("/interactions", interactionHandler.ListMine)
			meAdult.GET("/interactions/:interaction_id/messages", chatHandler.GetMessages)
			meAdult.GET("/interactions/:interaction_id/distance", distanceHandler.GetDistance)
			meAdult.POST("/upload/chat", uploadHandler.UploadChatMedia)
			meAdult.POST("/fcm-token", meHandler.RegisterFCMToken)
			meAdult.POST("/interactions/:interaction_id/video-call-request", interactionHandler.VideoCallRequest)
			meAdult.POST("/boost/initiate", middleware.RequireRole("COMPANION"), mpesaHandler.InitiateBoost)
		}
		api.POST("/payments/mpesa/initiate", authMw, adultMw, mpesaHandler.Initiate)
		api.POST("/interactions", authMw, adultMw, interactionHandler.Create)
		api.POST("/interactions/:id/accept", authMw, adultMw, middleware.RequireRole("COMPANION"), interactionHandler.Accept)
		api.POST("/interactions/:id/reject", authMw, adultMw, middleware.RequireRole("COMPANION"), interactionHandler.Reject)
		api.POST("/interactions/:id/service-done", authMw, adultMw, middleware.RequireRole("CLIENT"), interactionHandler.ServiceDone)
		api.POST("/favorites/:companion_id", authMw, adultMw, favoriteHandler.Add)
		api.DELETE("/favorites/:companion_id", authMw, adultMw, favoriteHandler.Remove)
		api.POST("/block/:user_id", authMw, adultMw, blockHandler.Block)
		api.DELETE("/block/:user_id", authMw, adultMw, blockHandler.Unblock)
		api.POST("/reports", authMw, adultMw, reportHandler.Create)

		companions := api.Group("/companions")
		companions.Use(authMw, adultMw, middleware.RequireRole("COMPANION"))
		{
			companions.PUT("/profile", companionHandler.UpdateProfile)
			companions.POST("/media", companionHandler.UploadMedia)
			companions.GET("/pricing", pricingHandler.List)
			companions.POST("/pricing", pricingHandler.Create)
			companions.PUT("/pricing/:id", pricingHandler.Update)
			companions.DELETE("/pricing/:id", pricingHandler.Delete)
			companions.POST("/boost", boostHandler.Activate)
		}
		api.POST("/webhooks/payment", paymentWebhookHandler.Handle)
		api.POST("/webhooks/mpesa", mpesaWebhookHandler.Handle)
		api.POST("/webhooks/withdrawal", withdrawalWebhookHandler.Handle)
	}

	r.GET("/ws/map", ws.UpgradeMapWS(&cfg.JWT, mapHub))
	r.GET("/ws/chat", handler.UpgradeChatWS(&cfg.JWT, chatHub, interactionRepo, userRepo, notifSvc))
	r.GET("/ws/video", handler.UpgradeVideoWS(&cfg.JWT, videoHub, interactionRepo))

	return r
}
