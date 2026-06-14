package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sameetpatro/go-qr-auth/internal/audit"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/database"
	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/notifications"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
	"github.com/sameetpatro/go-qr-auth/internal/service"
)

func main() {
	_ = godotenv.Load()

	csvPath := flag.String("file", "scripts/seed_guests.csv", "CSV file with guest rows")
	coordinators := flag.Int("coordinators", 2, "Number of coordinator accounts to create")
	reset := flag.Bool("reset", false, "Wipe all guest/coordinator data before seeding")
	flag.Parse()

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

	userRepo := repository.NewUserRepository(db)
	guestRepo := repository.NewGuestRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	auditService := audit.NewService(auditRepo)
	qrService := qr.NewService(cfg.Storage, cfg.Event, cfg.JWT.AccessSecret)
	notificationService := notifications.NewService(
		notifications.NewWhatsAppProvider(),
		notifications.NewEmailProvider(),
		notifications.NewSMSProvider(),
	)
	guestService := service.NewGuestService(guestRepo, userRepo, qrService, notificationService, cfg.Event, auditService, nil)
	coordinatorService := service.NewCoordinatorService(userRepo, auditService)
	leaderService := service.NewLeaderService(userRepo, auditService)
	resetService := service.NewResetService(db, cfg.Storage.QRImagePath)

	ctx := context.Background()
	master, err := userRepo.FindByEmail(ctx, "master@event.app")
	if err != nil || master == nil {
		log.Fatalf("master user not found — run migrations first")
	}

	if *reset {
		fmt.Println("Resetting all guest and coordinator data...")
		if err := resetService.ResetAllData(ctx); err != nil {
			log.Fatalf("reset: %v", err)
		}
		fmt.Println("Reset complete.")
	}

	seedLeaderID, err := ensureSeedLeader(ctx, leaderService, master)
	if err != nil {
		log.Fatalf("seed leader: %v", err)
	}

	importGuests(ctx, guestService, master, seedLeaderID, *csvPath)
	seedCoordinators(ctx, coordinatorService, master, *coordinators)
}

func ensureSeedLeader(ctx context.Context, leaderService *service.LeaderService, master *models.User) (int64, error) {
	leaders, err := leaderService.List(ctx)
	if err != nil {
		return 0, err
	}
	if len(leaders) > 0 {
		return leaders[0].ID, nil
	}
	gen := true
	resp, err := leaderService.Create(ctx, master.ID, dto.CreateLeaderRequest{
		Username:         "seedleader",
		GeneratePassword: gen,
	}, "127.0.0.1")
	if err != nil {
		return 0, err
	}
	fmt.Printf("Created seed leader: %s password=%s\n", resp.Email, resp.Password)
	return resp.ID, nil
}

func importGuests(ctx context.Context, guestService *service.GuestService, master *models.User, leaderID int64, csvPath string) {
	f, err := os.Open(csvPath)
	if err != nil {
		log.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	leaderIDPtr := leaderID
	result, err := guestService.Import(ctx, csvPath, f, master.ID, master.Role, &leaderIDPtr, "127.0.0.1")
	if err != nil {
		log.Fatalf("import guests: %v", err)
	}

	fmt.Printf("Guest import: %d/%d imported", result.Imported, result.TotalRows)
	if result.Failed > 0 {
		fmt.Printf(" (%d failed)", result.Failed)
	}
	fmt.Println()
	for _, e := range result.Errors {
		fmt.Printf("  - %s\n", e)
	}
}

func seedCoordinators(ctx context.Context, coordinatorService *service.CoordinatorService, master *models.User, count int) {
	fmt.Printf("Creating %d coordinator account(s)...\n", count)
	for i := 0; i < count; i++ {
		resp, err := coordinatorService.Create(ctx, master.ID, models.RoleMaster, "127.0.0.1")
		if err != nil {
			log.Fatalf("create coordinator: %v", err)
		}
		fmt.Printf("  Coordinator %d: email=%s password=%s\n", i+1, resp.Email, resp.Password)
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("Save these credentials — passwords are shown only once.")
}
