package qr

import (
	"bytes"
	"image/png"
	"testing"
)

func TestRenderInvitationCard(t *testing.T) {
	data, err := renderInvitationCard("test-token", GuestCardInfo{
		Name:          "Test Guest",
		EventName:     "FELICITATIONS & VEER GOKULA MANCHAN",
		EventDate:     "19 July 2026 | 1:00 PM",
		EventLocation: "Bharat Mandapam, New Delhi",
	})
	if err != nil {
		t.Fatalf("render card: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() < 300 || bounds.Dy() < 300 {
		t.Fatalf("unexpected card size: %dx%d", bounds.Dx(), bounds.Dy())
	}
}
