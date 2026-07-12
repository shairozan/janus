package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

// proposalTerms is the shared body for request/counter.
type proposalTerms struct {
	Tier         *string `json:"tier"`
	Seats        int     `json:"seats"`
	LicenseModel string  `json:"license_model"`
	ValidityDays int     `json:"validity_days"`
	Message      string  `json:"message"`
}

func (b proposalTerms) toInput() billing.RequestInput {
	return billing.RequestInput{Tier: b.Tier, Seats: b.Seats, LicenseModel: b.LicenseModel, ValidityDays: b.ValidityDays}
}

// ProposalRequest handles POST /api/v1/orgs/{id}/proposals (customer admin).
func ProposalRequest(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		var body proposalTerms
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		pr, err := svc.Request(r.Context(), admin.OrganizationID, admin.ID, body.toInput())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", pr.ID))

		writeJSON(w, http.StatusCreated, pr)
	}
}

// ProposalList handles GET /api/v1/orgs/{id}/proposals (customer admin).
func ProposalList(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		proposals, err := svc.List(r.Context(), admin.OrganizationID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load proposals")

			return
		}

		writeJSON(w, http.StatusOK, proposals)
	}
}

// ProposalGet handles GET /api/v1/orgs/{id}/proposals/{pid} (customer admin).
func ProposalGet(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		pid, ok := pathInt(r, "pid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid proposal id")

			return
		}

		pr, err := svc.Get(r.Context(), admin.OrganizationID, pid)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())

			return
		}

		writeJSON(w, http.StatusOK, pr)
	}
}

// ProposalCounter handles POST /api/v1/orgs/{id}/proposals/{pid}/counter.
func ProposalCounter(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		pid, ok := pathInt(r, "pid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid proposal id")

			return
		}

		var body proposalTerms
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		pr, err := svc.Counter(r.Context(), admin.OrganizationID, pid, body.toInput(), body.Message)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", pr.ID))

		writeJSON(w, http.StatusOK, pr)
	}
}

// proposalAction wraps Accept/Pay (both take orgID + proposal id).
func proposalAction(
	action func(ctx context.Context, orgID, proposalID int64) (*models.AgreementProposal, error),
	writeJSON jsonWriter, writeError errWriter,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		pid, ok := pathInt(r, "pid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid proposal id")

			return
		}

		pr, err := action(r.Context(), admin.OrganizationID, pid)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", pid))

		writeJSON(w, http.StatusOK, pr)
	}
}

// ProposalAccept handles POST /api/v1/orgs/{id}/proposals/{pid}/accept.
func ProposalAccept(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return proposalAction(svc.Accept, writeJSON, writeError)
}

// ProposalPay handles POST /api/v1/orgs/{id}/proposals/{pid}/pay.
func ProposalPay(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return proposalAction(svc.Pay, writeJSON, writeError)
}

// OfferBody is the staff offer body.
type OfferBody struct {
	LineItems []struct {
		PriceID  int64 `json:"price_id"`
		Quantity int   `json:"quantity"`
	} `json:"line_items"`
	Message string `json:"message"`
}

// ProposalOffer handles POST /api/v1/billing/proposals/{pid}/offer (Janus staff).
func ProposalOffer(svc *billing.ProposalService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, ok := pathInt(r, "pid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid proposal id")

			return
		}

		var body OfferBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		items := make([]billing.OfferLineItem, 0, len(body.LineItems))
		for _, li := range body.LineItems {
			items = append(items, billing.OfferLineItem{PriceID: li.PriceID, Quantity: li.Quantity})
		}

		pr, err := svc.Offer(r.Context(), pid, billing.OfferInput{LineItems: items, Message: body.Message})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		writeJSON(w, http.StatusOK, pr)
	}
}

// SeatPreview handles GET /api/v1/orgs/{id}/agreements/{aid}/seats/preview?add=N.
func SeatPreview(svc *billing.SeatService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		aid, ok := pathInt(r, "aid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid agreement id")

			return
		}

		addN, _ := strconv.Atoi(r.URL.Query().Get("add"))

		preview, err := svc.Preview(r.Context(), orgID, aid, addN)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		writeJSON(w, http.StatusOK, preview)
	}
}

// SeatAddBody is the POST seats body.
type SeatAddBody struct {
	Add int `json:"add"`
}

// SeatAdd handles POST /api/v1/orgs/{id}/agreements/{aid}/seats.
func SeatAdd(svc *billing.SeatService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		aid, ok := pathInt(r, "aid")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid agreement id")

			return
		}

		var body SeatAddBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		agr, err := svc.Add(r.Context(), orgID, aid, body.Add)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", aid))
		middleware.Enrich(r, "new_seats", agr.Seats)

		writeJSON(w, http.StatusOK, agr)
	}
}

// StripeWebhook handles POST /api/v1/billing/webhook (public, signature-verified).
func StripeWebhook(stripe billing.Stripe, svc *billing.ProposalService, logger *log.Logger, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")

			return
		}

		event, err := stripe.ConstructWebhookEvent(payload, r.Header.Get("Stripe-Signature"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid webhook signature")

			return
		}

		if event.Type == billing.WebhookInvoicePaid && event.ObjectID != "" {
			if fulfillErr := svc.FulfillBySubscription(r.Context(), event.ObjectID); fulfillErr != nil {
				logger.Printf("webhook: fulfil failed for subscription %s: %v", event.ObjectID, fulfillErr)
				writeError(w, http.StatusInternalServerError, "fulfilment failed")

				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
