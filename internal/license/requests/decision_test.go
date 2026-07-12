package requests_test

import (
	"testing"

	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/requests"
)

//nolint:unparam // test helper kept parameterized for readability
func domainRule(domain string) models.AutoAcceptanceRule {
	return models.AutoAcceptanceRule{
		RuleType:    models.AutoAcceptanceRuleTypeEmailDomain,
		MatchDomain: &domain,
		Enabled:     true,
	}
}

func thresholdRule(max int) models.AutoAcceptanceRule {
	return models.AutoAcceptanceRule{
		RuleType:     models.AutoAcceptanceRuleTypeSeatThreshold,
		MaxAutoSeats: &max,
		Enabled:      true,
	}
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name string
		in   requests.Input
		want requests.Outcome
	}{
		{
			name: "no seats → deny",
			in:   requests.Input{Email: "a@knomix.io", UsedSeats: 5, TotalSeats: 5, Rules: []models.AutoAcceptanceRule{domainRule("knomix.io")}},
			want: requests.OutcomeDenyNoSeats,
		},
		{
			name: "email domain match → approve",
			in:   requests.Input{Email: "a@knomix.io", UsedSeats: 1, TotalSeats: 10, Rules: []models.AutoAcceptanceRule{domainRule("knomix.io")}},
			want: requests.OutcomeApprove,
		},
		{
			name: "email domain mismatch, no other rule → manual",
			in:   requests.Input{Email: "a@evil.com", UsedSeats: 1, TotalSeats: 10, Rules: []models.AutoAcceptanceRule{domainRule("knomix.io")}},
			want: requests.OutcomeManual,
		},
		{
			name: "seat threshold under → approve",
			in:   requests.Input{Email: "a@x.com", UsedSeats: 3, TotalSeats: 10, Rules: []models.AutoAcceptanceRule{thresholdRule(5)}},
			want: requests.OutcomeApprove,
		},
		{
			name: "seat threshold reached → manual",
			in:   requests.Input{Email: "a@x.com", UsedSeats: 5, TotalSeats: 10, Rules: []models.AutoAcceptanceRule{thresholdRule(5)}},
			want: requests.OutcomeManual,
		},
		{
			name: "no rules → manual",
			in:   requests.Input{Email: "a@x.com", UsedSeats: 1, TotalSeats: 10},
			want: requests.OutcomeManual,
		},
		{
			name: "disabled rule ignored → manual",
			in: requests.Input{Email: "a@knomix.io", UsedSeats: 1, TotalSeats: 10, Rules: []models.AutoAcceptanceRule{
				func() models.AutoAcceptanceRule { r := domainRule("knomix.io"); r.Enabled = false; return r }(),
			}},
			want: requests.OutcomeManual,
		},
		{
			name: "case-insensitive domain match → approve",
			in:   requests.Input{Email: "A@Knomix.IO", UsedSeats: 0, TotalSeats: 1, Rules: []models.AutoAcceptanceRule{domainRule("knomix.io")}},
			want: requests.OutcomeApprove,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := requests.Decide(tc.in)
			if got.Outcome != tc.want {
				t.Fatalf("Decide outcome = %q (%s), want %q", got.Outcome, got.Reason, tc.want)
			}

			if got.Reason == "" {
				t.Fatal("decision must carry a reason")
			}
		})
	}
}
