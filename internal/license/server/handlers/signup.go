package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/pharmalytica/janus/internal/license/signup"
)

// signupMaxBody bounds the request read — the endpoint is public/unauthenticated.
const signupMaxBody = 1 << 20 // 1 MiB

const (
	orgNameMax     = 255
	contactNameMax = 255
)

// SignupRequest is the POST /api/v1/signup body: a prospect creating a brand-new
// organization and its first admin.
type SignupRequest struct {
	OrgName     string `json:"org_name"`
	AdminEmail  string `json:"admin_email"`
	ContactName string `json:"contact_name,omitempty"`
}

// SignupResponse is the (deliberately opaque) signup result. It never reveals
// whether the account already existed — the message is identical either way, so
// the endpoint is not an account-existence oracle.
type SignupResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	CustomerID string `json:"customer_id"`
}

// Signup handles POST /api/v1/signup. It is PUBLIC (unauthenticated) because a
// brand-new prospect has no Cognito identity yet. It wraps the idempotent
// signup.Service, which provisions the first admin in Cognito (Cognito emails the
// invite + temp password), persists the organization + admin, and creates the
// org's Stripe customer. Internal errors are logged server-side and returned to
// the caller as a generic message — never leak Cognito/DB/Stripe internals to an
// unauthenticated caller.
func Signup(svc *signup.Service, logger *log.Logger, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")

			return
		}

		var req SignupRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, signupMaxBody)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		orgName := strings.TrimSpace(req.OrgName)
		if orgName == "" || len(orgName) > orgNameMax {
			writeError(w, http.StatusBadRequest, "org_name is required (max 255 characters)")

			return
		}

		// Syntactic validation only — no network check (the IQ/OQ boundary).
		addr, err := mail.ParseAddress(strings.ToLower(strings.TrimSpace(req.AdminEmail)))
		if err != nil {
			writeError(w, http.StatusBadRequest, "admin_email must be a valid email address")

			return
		}

		contactName := strings.TrimSpace(req.ContactName)
		if len(contactName) > contactNameMax {
			contactName = contactName[:contactNameMax]
		}

		customerID, err := signup.GenerateCustomerID(orgName)
		if err != nil {
			logger.Printf("signup: generate customer id: %v", err)
			writeError(w, http.StatusInternalServerError, "signup failed")

			return
		}

		res, err := svc.SignUp(r.Context(), signup.Input{
			OrgName:     orgName,
			CustomerID:  customerID,
			AdminEmail:  addr.Address,
			ContactName: contactName,
		})
		if err != nil {
			logger.Printf("signup: provisioning failed for %q: %v", addr.Address, err)
			writeError(w, http.StatusInternalServerError, "signup failed")

			return
		}

		writeJSON(w, http.StatusCreated, SignupResponse{
			Status:     "ok",
			Message:    "Check your email to finish setting up your account.",
			CustomerID: res.Organization.CustomerID,
		})
	}
}
