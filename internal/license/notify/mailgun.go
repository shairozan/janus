package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

// MailgunSender sends email via Mailgun. The domain + sending region are
// non-secret config; the API key is the secret (SOPS).
type MailgunSender struct {
	mg   *mailgun.MailgunImpl
	from string
}

// NewMailgunSender creates a Mailgun-backed Sender. region is the Mailgun
// sending region ("us" or "eu"); from is the sender address.
func NewMailgunSender(domain, apiKey, region, from string) *MailgunSender {
	mg := mailgun.NewMailgun(domain, apiKey)
	if region == "eu" {
		mg.SetAPIBase(mailgun.APIBaseEU)
	}

	return &MailgunSender{mg: mg, from: from}
}

// Send delivers a plain-text message to the recipients.
func (s *MailgunSender) Send(ctx context.Context, to []string, subject, body string) error {
	msg := mailgun.NewMessage(s.from, subject, body, to...)

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if _, _, err := s.mg.Send(sendCtx, msg); err != nil {
		return fmt.Errorf("mailgun send: %w", err)
	}

	return nil
}
