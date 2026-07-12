// Package notify sends operational notifications — primarily alerting customer
// admins when a license auto-allocation fails (reqs. 11 & 14).
package notify

import (
	"context"
	"fmt"
	"log"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Sender delivers an email. The Mailgun implementation satisfies it; tests use a
// fake (no live Mailgun — IQ/OQ boundary).
type Sender interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

// AdminNotifier notifies an organization's customer admins. It satisfies the
// portal's Notifier interface (NotifyAllocationFailure).
type AdminNotifier struct {
	db     *db.DB
	sender Sender
	logger *log.Logger
}

// NewAdminNotifier creates an AdminNotifier.
func NewAdminNotifier(database *db.DB, sender Sender, logger *log.Logger) *AdminNotifier {
	return &AdminNotifier{db: database, sender: sender, logger: logger}
}

// NotifyAllocationFailure emails all active customer admins of the org. It is
// fire-and-forget: failures are logged, not returned, so they never block the
// request that triggered them.
func (n *AdminNotifier) NotifyAllocationFailure(ctx context.Context, orgID int64, reason string) {
	admins, err := n.adminEmails(ctx, orgID)
	if err != nil {
		n.logger.Printf("notify: failed to load admins for org %d: %v", orgID, err)

		return
	}

	if len(admins) == 0 {
		n.logger.Printf("notify: org %d has no active admins to notify", orgID)

		return
	}

	subject := "Action required: a Janus license request could not be auto-approved"
	body := fmt.Sprintf(
		"A license request for your organization could not be automatically approved.\n\nReason: %s\n\n"+
			"Please review the request queue in the Janus management portal and add seats or approve manually.",
		reason,
	)

	if sendErr := n.sender.Send(ctx, admins, subject, body); sendErr != nil {
		n.logger.Printf("notify: failed to send allocation-failure email for org %d: %v", orgID, sendErr)
	}
}

func (n *AdminNotifier) adminEmails(ctx context.Context, orgID int64) ([]string, error) {
	var admins []models.OrgUser
	if err := n.db.DB.WithContext(ctx).
		Where("organization_id = ? AND role = ? AND deactivated_at IS NULL", orgID, models.OrgUserRoleCustomerAdmin).
		Find(&admins).Error; err != nil {
		return nil, err
	}

	emails := make([]string, 0, len(admins))
	for i := range admins {
		if admins[i].Email != "" {
			emails = append(emails, admins[i].Email)
		}
	}

	return emails, nil
}
