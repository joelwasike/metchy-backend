package router

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"lusty/config"
	"lusty/internal/domain"
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
	// CORS for dashboard
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	})
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

	referralRepo := repository.NewReferralRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	adminRepo := repository.NewAdminRepository(db)

	// Seed default system settings
	_ = settingRepo.SeedDefaults(map[string]string{
		domain.SettingReferralBonusReferrer:  "10000", // KES 100
		domain.SettingReferralBonusReferred:  "20000", // KES 200
		domain.SettingReferralCommissionRate: "0.05",
		domain.SettingReferralMaxTx:          "2",
	})

	referralSvc := service.NewReferralService(referralRepo, walletRepo, settingRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, presenceRepo, auditRepo, companionRepo, referralSvc)
	meHandler := handler.NewMeHandler(userRepo, companionRepo, locRepo, favRepo, paymentRepo, interactionRepo, walletRepo, notifSvc)
	googleOAuthHandler := handler.NewGoogleOAuthHandler(cfg, authSvc, presenceRepo, auditRepo, companionRepo, referralSvc)
	appleOAuthHandler := handler.NewAppleOAuthHandler(authSvc, presenceRepo, auditRepo, companionRepo, referralSvc)
	adminHandler := handler.NewAdminHandler(adminRepo, settingRepo, authSvc)
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
	interactionHandler := handler.NewInteractionHandler(interactionRepo, companionRepo, paymentRepo, walletRepo, userRepo, notifSvc, referralRepo)
	paymentWebhookHandler := handler.NewPaymentWebhookHandler(paymentRepo, auditRepo, notifSvc, cfg)
	walletHandler := handler.NewWalletHandler(walletRepo)
	mpesaHandler := handler.NewMpesaHandler(cfg, paymentRepo, interactionRepo, companionRepo, walletRepo, userRepo, notifSvc)
	mpesaWebhookHandler := handler.NewMpesaWebhookHandler(paymentRepo, interactionRepo, companionRepo, walletRepo, auditRepo, notifSvc, userRepo, referralRepo)
	withdrawalRepo := repository.NewWithdrawalRepository(db)
	withdrawalHandler := handler.NewWithdrawalHandler(cfg, walletRepo, withdrawalRepo, companionRepo)
	withdrawalWebhookHandler := handler.NewWithdrawalWebhookHandler(withdrawalRepo, walletRepo)
	chatHandler := handler.NewChatHandler(interactionRepo, companionRepo)
	uploadHandler := handler.NewUploadHandler(cloud)
	distanceHandler := handler.NewDistanceHandler(interactionRepo, companionRepo, locRepo, userRepo)
	referralHandler := handler.NewReferralHandler(referralRepo)
	cryptoHandler := handler.NewCryptoHandler(cfg, paymentRepo, companionRepo, walletRepo, userRepo, notifSvc)
	cryptoWebhookHandler := handler.NewCryptoWebhookHandler(paymentRepo, interactionRepo, companionRepo, walletRepo, userRepo, notifSvc, referralRepo)

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
			authGroup.POST("/apple/token", appleOAuthHandler.Token)
		}

		api.GET("/discover", authMw, adultMw, discoveryHandler.Discover)
		api.GET("/companions/:id", authMw, adultMw, companionHandler.GetProfile)

		me := api.Group("/me")
		me.Use(authMw)
		{
			me.GET("/profile", meHandler.GetProfile)
			me.POST("/onboarding/complete", meHandler.CompleteOnboarding) // no adult - may set DOB for Google signups
			me.POST("/fcm-token", meHandler.RegisterFCMToken)             // no adult - must work before DOB is set
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
			meAdult.POST("/kyc-complete", meHandler.CompleteKYC)
			meAdult.POST("/interactions/:interaction_id/video-call-request", interactionHandler.VideoCallRequest)
			meAdult.POST("/boost/initiate", middleware.RequireRole("COMPANION"), mpesaHandler.InitiateBoost)
			meAdult.GET("/referral-code", referralHandler.GetMyReferralCode)
			meAdult.GET("/referrals", referralHandler.GetMyReferrals)
		}
		api.POST("/payments/mpesa/initiate", authMw, adultMw, mpesaHandler.Initiate)
		api.GET("/payments/crypto/rates", authMw, adultMw, cryptoHandler.GetRates)
		api.POST("/payments/crypto/initiate", authMw, adultMw, cryptoHandler.Initiate)
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
		api.POST("/webhooks/crypto", cryptoWebhookHandler.Handle)
		api.POST("/webhooks/withdrawal", withdrawalWebhookHandler.Handle)
	}

	r.GET("/ws/map", ws.UpgradeMapWS(&cfg.JWT, mapHub))
	r.GET("/ws/chat", handler.UpgradeChatWS(&cfg.JWT, chatHub, interactionRepo, userRepo, notifSvc))
	r.GET("/ws/video", handler.UpgradeVideoWS(&cfg.JWT, videoHub, interactionRepo))

	// Admin API routes
	adminAPI := r.Group("/api/v1/admin")
	{
		adminAPI.POST("/login", adminHandler.AdminLogin)
	}
	adminAuth := adminAPI.Group("")
	adminAuth.Use(authMw, middleware.AdminRequired())
	{
		adminAuth.GET("/dashboard", adminHandler.Dashboard)
		adminAuth.GET("/users", adminHandler.ListUsers)
		adminAuth.GET("/users/:id", adminHandler.GetUser)
		adminAuth.PATCH("/users/:id", adminHandler.UpdateUser)
		adminAuth.GET("/companions", adminHandler.ListCompanions)
		adminAuth.GET("/transactions", adminHandler.ListTransactions)
		adminAuth.GET("/payments", adminHandler.ListPayments)
		adminAuth.GET("/withdrawals", adminHandler.ListWithdrawals)
		adminAuth.GET("/interactions", adminHandler.ListInteractions)
		adminAuth.GET("/reports", adminHandler.ListReports)
		adminAuth.PATCH("/reports/:id", adminHandler.UpdateReport)
		adminAuth.GET("/referrals", adminHandler.ListReferrals)
		adminAuth.GET("/online-users", adminHandler.ListOnlineUsers)
		adminAuth.GET("/settings", adminHandler.GetSettings)
		adminAuth.PUT("/settings", adminHandler.UpdateSettings)
		adminAuth.GET("/analytics", adminHandler.Analytics)
	}

	// Serve dashboard static files (built React app)
	dashboardDir := "dashboard/dist"
	if _, err := os.Stat(dashboardDir); err == nil {
		r.Static("/dashboard/assets", dashboardDir+"/assets")
		r.GET("/dashboard/*filepath", func(c *gin.Context) {
			fp := c.Param("filepath")
			fullPath := dashboardDir + fp
			if _, err := os.Stat(fullPath); err != nil || strings.HasSuffix(fp, "/") || fp == "" {
				c.File(dashboardDir + "/index.html")
				return
			}
			c.File(fullPath)
		})
	}

	return r
}
