// @title Event Entry Management API
// @version 1.0
// @description QR-based Event Entry Management Platform API
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	jwtsvc "github.com/sameetpatro/go-qr-auth/internal/auth"
	"github.com/joho/godotenv"
	"github.com/sameetpatro/go-qr-auth/internal/audit"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/database"
	"github.com/sameetpatro/go-qr-auth/internal/handler"
	"github.com/sameetpatro/go-qr-auth/internal/notifications"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
	"github.com/sameetpatro/go-qr-auth/internal/router"
	"github.com/sameetpatro/go-qr-auth/internal/service"
	"github.com/sameetpatro/go-qr-auth/internal/storage"
	"github.com/sameetpatro/go-qr-auth/internal/websocket"

	_ "github.com/sameetpatro/go-qr-auth/docs"
)

func main() {
	// Load .env from the working dir (local dev) or from Render's secret-file
	// mount (add the file as a Secret File named ".env" in the dashboard).
	// Neither call overrides variables already set in the environment.
	_ = godotenv.Load()
	_ = godotenv.Load("/etc/secrets/.env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("migration: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	guestRepo := repository.NewGuestRepository(db)
	scanRepo := repository.NewScanRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	analyticsRepo := repository.NewAnalyticsRepository(db)

	// Core services
	jwtService := jwtsvc.NewService(cfg.JWT)

	var qrUploader qr.Uploader
	if cfg.Storage.CloudinaryURL != "" {
		cld, err := storage.NewCloudinary(cfg.Storage.CloudinaryURL)
		if err != nil {
			log.Fatalf("cloudinary: %v", err)
		}
		qrUploader = cld
		log.Println("QR image storage: Cloudinary (permanent CDN)")
	} else {
		log.Printf("QR image storage: local disk at %s (set CLOUDINARY_URL for permanent storage)", cfg.Storage.QRImagePath)
	}
	qrService := qr.NewService(cfg.Storage, cfg.Event, cfg.JWT.AccessSecret, qrUploader)
	auditService := audit.NewService(auditRepo)
	notificationService := notifications.NewService(
		notifications.NewWhatsAppProvider(),
		notifications.NewEmailProvider(),
		notifications.NewSMSProvider(),
	)

	wsHub := websocket.NewHub()
	go wsHub.Run()
	wsBroadcaster := websocket.NewBroadcaster(wsHub)

	authService := service.NewAuthService(userRepo, tokenRepo, jwtService, auditService)
	coordinatorService := service.NewCoordinatorService(userRepo, auditService)
	leaderService := service.NewLeaderService(userRepo, auditService)
	guestService := service.NewGuestService(guestRepo, userRepo, qrService, notificationService, cfg.Event, auditService, wsBroadcaster)
	service.SetAPIBaseURL(cfg.Server.BaseURL)
	scanService := service.NewScanService(guestRepo, scanRepo, auditService, wsBroadcaster)
	analyticsService := service.NewAnalyticsService(analyticsRepo)
	insightsService := service.NewInsightsService(analyticsRepo)
	reportService := service.NewReportService(guestRepo, analyticsRepo, auditService)
	resetService := service.NewResetService(db, cfg.Storage.QRImagePath)

	// Handlers
	handlers := router.Handlers{
		Auth:        handler.NewAuthHandler(authService),
		Coordinator: handler.NewCoordinatorHandler(coordinatorService, wsBroadcaster),
		Leader:      handler.NewLeaderHandler(leaderService, wsBroadcaster),
		Guest:       handler.NewGuestHandler(guestService),
		Scan:        handler.NewScanHandler(scanService),
		Analytics:   handler.NewAnalyticsHandler(analyticsService, insightsService, reportService),
		Admin:       handler.NewAdminHandler(resetService),
		WSHub:       wsHub,
		JWT:         jwtService,
		Config:      cfg,
	}

	engine := router.Setup(handlers)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("Server stopped")
}
