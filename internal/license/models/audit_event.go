package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Audit resources (nouns).
const (
	AuditResourceLicense   = "license"
	AuditResourceAgreement = "agreement"
	AuditResourceMember    = "member"
	AuditResourcePublicKey = "public_key"
	AuditResourceSSOConfig = "sso_config"
	AuditResourceProposal  = "proposal"
	AuditResourceSeat      = "seat"
)

// Audit actions (verbs).
const (
	AuditActionAllocate = "allocate"
	AuditActionApprove  = "approve"
	AuditActionReject   = "reject"
	AuditActionCreate   = "create"
	AuditActionReplace  = "replace"
	AuditActionOffboard = "offboard"
	AuditActionPromote  = "promote"
)

// AuditEvent is one append-only record of a portal action. It is captured
// automatically by the Audit middleware (see A9): noun/verb + the redacted
// request body. UUIDv7 keyed (time-ordered) for index locality at scale; the
// table is month range-partitioned on occurred_at.
type AuditEvent struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OccurredAt     time.Time  `gorm:"not null;default:now();primaryKey" json:"occurred_at"`
	OrganizationID *int64     `gorm:"index:idx_audit_events_org_time" json:"organization_id,omitempty"`
	ActorOrgUserID *int64     `gorm:"index:idx_audit_events_actor" json:"actor_org_user_id,omitempty"`
	ActorLabel     string     `gorm:"type:varchar(255)" json:"actor_label"`
	Resource       string     `gorm:"type:varchar(50);not null" json:"resource"`
	Action         string     `gorm:"type:varchar(50);not null" json:"action"`
	ResourceID     *string    `gorm:"type:varchar(255)" json:"resource_id,omitempty"`
	RequestMethod  string     `gorm:"type:varchar(10)" json:"request_method"`
	RequestPath    string     `gorm:"type:varchar(1024)" json:"request_path"`
	ResultStatus   int        `json:"result_status"`
	Body           JSONObject `gorm:"type:jsonb" json:"body,omitempty"`
}

// TableName overrides the default table name.
func (AuditEvent) TableName() string {
	return "audit_events"
}

// BeforeCreate assigns a time-ordered UUIDv7 id and stamps OccurredAt if unset,
// so callers (the Audit middleware) don't have to.
func (e *AuditEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}

		e.ID = id
	}

	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}

	return nil
}
