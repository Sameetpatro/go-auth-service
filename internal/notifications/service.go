package notifications

import (
	"context"
	"fmt"
	"log"
)

type Message struct {
	To          string
	Subject     string
	Body        string
	Attachment  *Attachment
}

type Attachment struct {
	URL      string
	Filename string
	MimeType string
}

type Provider interface {
	Name() string
	SendMessage(ctx context.Context, msg Message) error
	SendAttachment(ctx context.Context, msg Message) error
}

type Service struct {
	providers map[string]Provider
	defaultCh string
}

func NewService(providers ...Provider) *Service {
	m := make(map[string]Provider)
	for _, p := range providers {
		m[p.Name()] = p
	}
	defaultCh := "whatsapp"
	if _, ok := m[defaultCh]; !ok && len(providers) > 0 {
		defaultCh = providers[0].Name()
	}
	return &Service{providers: m, defaultCh: defaultCh}
}

func (s *Service) Send(ctx context.Context, channel string, msg Message) error {
	p, ok := s.providers[channel]
	if !ok {
		return fmt.Errorf("notification provider %q not found", channel)
	}
	if msg.Attachment != nil {
		return p.SendAttachment(ctx, msg)
	}
	return p.SendMessage(ctx, msg)
}

func BuildGuestInvitationMessage(guestName, eventName, eventDate, eventLocation, qrImageURL string) string {
	msg := fmt.Sprintf(
		"Hello %s!\n\nYou are invited to %s\nDate: %s\nLocation: %s",
		guestName, eventName, eventDate, eventLocation,
	)
	if qrImageURL != "" {
		msg += fmt.Sprintf("\n\nYour invitation QR code:\n%s", qrImageURL)
	}
	msg += "\n\nPlease present your QR code at the venue."
	return msg
}

func (s *Service) SendGuestInvitation(ctx context.Context, guestName, phone, eventName, eventDate, eventLocation, qrImageURL string) error {
	body := BuildGuestInvitationMessage(guestName, eventName, eventDate, eventLocation, qrImageURL)
	msg := Message{
		To:      phone,
		Subject: fmt.Sprintf("Invitation to %s", eventName),
		Body:    body,
		Attachment: &Attachment{
			URL:      qrImageURL,
			Filename: "invitation_qr.png",
			MimeType: "image/png",
		},
	}
	if phone == "" {
		log.Printf("[notification] skipping invitation for %s: no phone number", guestName)
		return nil
	}
	return s.Send(ctx, s.defaultCh, msg)
}

// WhatsAppProvider is a stub for WhatsApp Business API integration.
type WhatsAppProvider struct{}

func NewWhatsAppProvider() *WhatsAppProvider { return &WhatsAppProvider{} }

func (p *WhatsAppProvider) Name() string { return "whatsapp" }

func (p *WhatsAppProvider) SendMessage(ctx context.Context, msg Message) error {
	log.Printf("[whatsapp] to=%s body=%q", msg.To, msg.Body)
	return nil
}

func (p *WhatsAppProvider) SendAttachment(ctx context.Context, msg Message) error {
	log.Printf("[whatsapp] to=%s attachment=%s body=%q", msg.To, msg.Attachment.URL, msg.Body)
	return nil
}

// EmailProvider is a stub for email integration.
type EmailProvider struct{}

func NewEmailProvider() *EmailProvider { return &EmailProvider{} }

func (p *EmailProvider) Name() string { return "email" }

func (p *EmailProvider) SendMessage(ctx context.Context, msg Message) error {
	log.Printf("[email] to=%s subject=%q", msg.To, msg.Subject)
	return nil
}

func (p *EmailProvider) SendAttachment(ctx context.Context, msg Message) error {
	log.Printf("[email] to=%s attachment=%s", msg.To, msg.Attachment.URL)
	return nil
}

// SMSProvider is a stub for SMS integration.
type SMSProvider struct{}

func NewSMSProvider() *SMSProvider { return &SMSProvider{} }

func (p *SMSProvider) Name() string { return "sms" }

func (p *SMSProvider) SendMessage(ctx context.Context, msg Message) error {
	log.Printf("[sms] to=%s body=%q", msg.To, msg.Body)
	return nil
}

func (p *SMSProvider) SendAttachment(ctx context.Context, msg Message) error {
	log.Printf("[sms] to=%s (attachments not supported)", msg.To)
	return p.SendMessage(ctx, msg)
}
