package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/database"
	"github.com/sameetpatro/go-qr-auth/internal/service"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	resetService := service.NewResetService(db, cfg.Storage.QRImagePath)
	if err := resetService.ResetAllData(context.Background()); err != nil {
		log.Fatalf("reset: %v", err)
	}

	fmt.Println("All event data has been reset (master account kept).")
}
