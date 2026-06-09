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
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/notifications"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
	"github.com/sameetpatro/go-qr-auth/internal/service"
)

func main() {
	_ = godotenv.Load()

	csvPath := flag.String("file", "scripts/seed_guests.csv", "CSV file with guest rows")
	coordinators := flag.Int("coordinators", 2, "Number of coordinator accounts to create if none exist")
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
	guestService := service.NewGuestService(guestRepo, qrService, notificationService, cfg.Event, auditService, nil)
	coordinatorService := service.NewCoordinatorService(userRepo, auditService)

	ctx := context.Background()
	master, err := userRepo.FindByEmail(ctx, "master@event.app")
	if err != nil || master == nil {
		log.Fatalf("master user not found — run migrations first")
	}

	importGuests(ctx, guestService, master, *csvPath)
	seedCoordinators(ctx, coordinatorService, userRepo, master, *coordinators)
}

func importGuests(ctx context.Context, guestService *service.GuestService, master *models.User, csvPath string) {
	f, err := os.Open(csvPath)
	if err != nil {
		log.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	result, err := guestService.Import(ctx, csvPath, f, master.ID, master.Role, "127.0.0.1")
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

func seedCoordinators(ctx context.Context, coordinatorService *service.CoordinatorService, userRepo *repository.UserRepository, master *models.User, count int) {
	existing, err := userRepo.ListCoordinators(ctx)
	if err != nil {
		log.Fatalf("list coordinators: %v", err)
	}
	if len(existing) > 0 {
		fmt.Printf("Coordinators already exist (%d) — skipping creation\n", len(existing))
		for _, c := range existing {
			status := "active"
			if !c.IsActive {
				status = "disabled"
			}
			fmt.Printf("  - %s (%s)\n", c.Email, status)
		}
		return
	}

	fmt.Printf("Creating %d coordinator account(s)...\n", count)
	for i := 0; i < count; i++ {
		resp, err := coordinatorService.Create(ctx, master.ID, "127.0.0.1")
		if err != nil {
			log.Fatalf("create coordinator: %v", err)
		}
		fmt.Printf("  Coordinator %d: email=%s password=%s\n", i+1, resp.Email, resp.Password)
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("Save these credentials — passwords are shown only once.")
}
