// Package requests implements license-request decisioning (auto-acceptance rule
// evaluation) and seat accounting for the management portal.
package requests

import (
	"strings"

	"github.com/pharmalytica/janus/internal/license/models"
)

// Outcome is the result of evaluating a license request.
type Outcome string

const (
	// OutcomeApprove means an auto-acceptance rule matched.
	OutcomeApprove Outcome = "approve"
	// OutcomeManual means no rule matched; a customer admin must decide.
	OutcomeManual Outcome = "manual"
	// OutcomeDenyNoSeats means the agreement has no available seats. This is an
	// allocation failure: customer admins must be notified (reqs. 11 & 14).
	OutcomeDenyNoSeats Outcome = "deny_no_seats"
)

// Decision is the result of Decide.
type Decision struct {
	Outcome Outcome
	Reason  string
}

// Input is the (pure) input to a decision.
type Input struct {
	Email      string
	Rules      []models.AutoAcceptanceRule
	UsedSeats  int
	TotalSeats int
}

// Decide evaluates a license request against the org's auto-acceptance rules and
// current seat usage. It is a pure function — all I/O (rule load, seat count)
// happens at the caller.
func Decide(in Input) Decision {
	if in.UsedSeats >= in.TotalSeats {
		return Decision{Outcome: OutcomeDenyNoSeats, Reason: "no seats available on this agreement"}
	}

	for i := range in.Rules {
		rule := in.Rules[i]
		if !rule.IsActive() {
			continue
		}

		switch rule.RuleType {
		case models.AutoAcceptanceRuleTypeEmailDomain:
			if rule.MatchDomain != nil && emailDomainMatches(in.Email, *rule.MatchDomain) {
				return Decision{Outcome: OutcomeApprove, Reason: "auto-approved: email domain matched"}
			}
		case models.AutoAcceptanceRuleTypeSeatThreshold:
			if rule.MaxAutoSeats != nil && in.UsedSeats < *rule.MaxAutoSeats {
				return Decision{Outcome: OutcomeApprove, Reason: "auto-approved: under seat threshold"}
			}
		}
	}

	return Decision{Outcome: OutcomeManual, Reason: "no auto-acceptance rule matched; manual review required"}
}

func emailDomainMatches(email, domain string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false
	}

	return strings.EqualFold(parts[1], domain)
}
