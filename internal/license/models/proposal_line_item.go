package models

// ProposalLineItem is one priced line of an AgreementProposal. PriceID references
// the catalog StripePrice row (stripe_prices.id), never a hardcoded literal.
// AmountCents is a snapshot at proposal time.
type ProposalLineItem struct {
	ID          int64 `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	ProposalID  int64 `gorm:"not null;index:idx_proposal_line_items_proposal" json:"proposal_id" db:"proposal_id"`
	PriceID     int64 `gorm:"not null;column:price_id" json:"price_id" db:"price_id"`
	Quantity    int   `gorm:"not null" json:"quantity" db:"quantity"`
	AmountCents int   `gorm:"not null" json:"amount_cents" db:"amount_cents"`

	// Relationships — belongs-to the catalog price (stripe_prices.id).
	Price *StripePrice `gorm:"foreignKey:PriceID;references:ID" json:"price,omitempty"`
}

// TableName overrides the default table name.
func (ProposalLineItem) TableName() string {
	return "proposal_line_items"
}
