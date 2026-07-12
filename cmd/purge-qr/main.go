// Command purge-qr deletes stored QR invitation cards from Cloudinary.
//
// By default it removes everything under the "qr/" prefix (the folder this app
// uploads to). Pass -prefix to target a different folder, or -all to wipe every
// image in the Cloudinary account.
//
//	go run ./cmd/purge-qr            # delete qr/ folder
//	go run ./cmd/purge-qr -prefix x/ # delete x/ folder
//	go run ./cmd/purge-qr -all       # delete ALL images in the account
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/storage"
)

func main() {
	prefix := flag.String("prefix", "qr/", "asset prefix/folder to delete")
	all := flag.Bool("all", false, "delete ALL images in the Cloudinary account (ignores -prefix)")
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.Storage.CloudinaryURL == "" {
		log.Fatal("CLOUDINARY_URL is not set; nothing to purge")
	}

	cld, err := storage.NewCloudinary(cfg.Storage.CloudinaryURL)
	if err != nil {
		log.Fatalf("cloudinary: %v", err)
	}

	var (
		count int
		err2  error
	)
	if *all {
		fmt.Println("Purging ALL images from Cloudinary...")
		count, err2 = cld.PurgeEverything(context.Background())
	} else {
		fmt.Printf("Purging prefix %q from Cloudinary...\n", *prefix)
		count, err2 = cld.PurgeAll(context.Background(), *prefix)
	}
	if err2 != nil {
		log.Fatalf("purge failed after deleting %d asset(s): %v", count, err2)
	}

	fmt.Printf("Done. Deleted %d asset(s).\n", count)
}
