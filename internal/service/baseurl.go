package service

import (
	"fmt"
	"strings"
)

var apiBaseURL = "http://localhost:8080"

// SetAPIBaseURL configures absolute URLs returned in guest API responses.
func SetAPIBaseURL(url string) {
	if strings.TrimSpace(url) != "" {
		apiBaseURL = strings.TrimRight(strings.TrimSpace(url), "/")
	}
}

func guestQRImageURL(guestID int64) string {
	return fmt.Sprintf("%s/api/v1/guests/%d/qr-image", apiBaseURL, guestID)
}
