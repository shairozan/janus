package models

import (
	"time"
)

// AgreementProposal statuses.
const (
	ProposalStatusRequested = "requested"
	ProposalStatusProposed  = "proposed"
	ProposalStatusCountered = "countered"
	ProposalStatusAccepted  = "accepted"
	ProposalStatusPaid      = "paid"
	ProposalStatusFulfilled = "fulfilled"
	ProposalStatusRejected  = "rejected"
	ProposalStatusExpired   = "expired"
)

// DefaultProposalValidityDays is the default license term basis (one calendar
// year) — a negotiable line that sets the agreement Start/EndDate and thus the
// license exp.
const DefaultProposalValidityDays = 365

// AgreementProposal models the "customer requests agreement → Janus proposes
// (with price identifiers) → negotiate or pay" lifecycle, distinct from the
// per-user LicenseRequest. A counter creates a new versioned row linked via
// ParentProposalID; on fulfilment it creates/activates the Agreement.
type AgreementProposal struct {
	ID                   int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID       int64      `gorm:"not null;index:idx_proposals_org" json:"organization_id" db:"organization_id"`
	RequestedByUserID    int64      `gorm:"not null" json:"requested_by_user_id" db:"requested_by_user_id"`
	Status               string     `gorm:"type:varchar(20);not null" json:"status" db:"status"`
	Tier                 *string    `gorm:"type:varchar(50)" json:"tier,omitempty" db:"tier"`
	Seats                int        `gorm:"not null" json:"seats" db:"seats"`
	LicenseModel         string     `gorm:"type:varchar(20);not null" json:"license_model" db:"license_model"`
	ValidityDays         int        `gorm:"not null;default:365" json:"validity_days" db:"validity_days"`
	TotalAmountCents     int        `gorm:"not null;default:0" json:"total_amount_cents" db:"total_amount_cents"`
	StripeQuoteID        *string    `gorm:"type:varchar(255)" json:"stripe_quote_id,omitempty" db:"stripe_quote_id"`
	ValidUntil           *time.Time `gorm:"default:null" json:"valid_until,omitempty" db:"valid_until"`
	Message              *string    `gorm:"type:text" json:"message,omitempty" db:"message"`
	ParentProposalID     *int64     `gorm:"default:null" json:"parent_proposal_id,omitempty" db:"parent_proposal_id"`
	ResultingAgreementID *int64     `gorm:"default:null" json:"resulting_agreement_id,omitempty" db:"resulting_agreement_id"`
	CreatedAt            time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `gorm:"not null;default:now()" json:"updated_at" db:"updated_at"`

	// Relationships
	Organization *Organization      `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	LineItems    []ProposalLineItem `gorm:"foreignKey:ProposalID" json:"line_items,omitempty"`
}

// TableName overrides the default table name.
func (AgreementProposal) TableName() string {
	return "agreement_proposals"
}
