package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// maxAuditBodyBytes caps how much of a request body is captured into an audit record.
const maxAuditBodyBytes = 64 * 1024

// contextKeyAuditState holds the per-request enrichment state.
const contextKeyAuditState contextKey = "audit_state"

// defaultRedactKeys are substrings whose matching JSON keys are redacted from
// captured bodies. Substring match, case-insensitive (so "token" covers
// access_token/id_token, "secret" covers client_secret, etc.).
var defaultRedactKeys = []string{"password", "secret", "token", "card", "cvc", "cvv"}

// AuditSink persists an audit event.
type AuditSink interface {
	Write(ctx context.Context, e *models.AuditEvent) error
}

// AuditSinkFunc adapts a function to an AuditSink.
type AuditSinkFunc func(ctx context.Context, e *models.AuditEvent) error

// Write calls the function.
func (f AuditSinkFunc) Write(ctx context.Context, e *models.AuditEvent) error {
	return f(ctx, e)
}

// NewGormAuditSink returns an AuditSink that appends events to the audit_events
// table. The AuditEvent.BeforeCreate hook assigns the UUIDv7 id.
func NewGormAuditSink(database *db.DB) AuditSink {
	return AuditSinkFunc(func(ctx context.Context, e *models.AuditEvent) error {
		return database.DB.WithContext(ctx).Create(e).Error
	})
}

// AuditTag labels a route with its resource (noun) and action (verb). Set
// AuditReads to also audit GETs (sensitive reads); by default only mutations
// (POST/PUT/PATCH/DELETE) are recorded.
type AuditTag struct {
	Resource   string
	Action     string
	AuditReads bool
}

// Audit returns middleware that records exactly one AuditEvent per mutating
// request (or sensitive read) for the tagged route. Capture is automatic — the
// handler needs no audit code — and handlers may add domain detail via Enrich.
// extraRedact adds route-specific keys to the secret denylist.
func Audit(tag AuditTag, sink AuditSink, logger *log.Logger, extraRedact ...string) Middleware {
	redactKeys := append(append([]string{}, defaultRedactKeys...), extraRedact...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && !tag.AuditReads {
				next.ServeHTTP(w, r)

				return
			}

			// Buffer the body so we can audit it AND let the handler read it.
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(io.LimitReader(r.Body, maxAuditBodyBytes))
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			st := &auditState{detail: map[string]interface{}{}}
			r = r.WithContext(context.WithValue(r.Context(), contextKeyAuditState, st))

			wrapped := newResponseWriter(w)
			next.ServeHTTP(wrapped, r)

			ev := buildAuditEvent(r, tag, bodyBytes, redactKeys, wrapped.statusCode, st)

			// Detach from the request context so the write isn't canceled when the
			// response completes; values (tracing, etc.) are preserved.
			//nolint:contextcheck // intentional: detached audit write outlives the request
			if err := sink.Write(context.WithoutCancel(r.Context()), ev); err != nil {
				logger.Printf("audit write failed (%s %s): %v", r.Method, r.URL.Path, err)
			}
		})
	}
}

// auditState accumulates handler-supplied detail for the current request.
type auditState struct {
	mu         sync.Mutex
	detail     map[string]interface{}
	resourceID string
}

// Enrich attaches a key/value to the current request's audit record. It is a
// no-op if the request is not being audited.
func Enrich(r *http.Request, key string, value interface{}) {
	if st, ok := r.Context().Value(contextKeyAuditState).(*auditState); ok {
		st.mu.Lock()
		st.detail[key] = value
		st.mu.Unlock()
	}
}

// SetAuditResourceID records the id of the entity the action affected.
func SetAuditResourceID(r *http.Request, id string) {
	if st, ok := r.Context().Value(contextKeyAuditState).(*auditState); ok {
		st.mu.Lock()
		st.resourceID = id
		st.mu.Unlock()
	}
}

func buildAuditEvent(r *http.Request, tag AuditTag, bodyBytes []byte, redactKeys []string, status int, st *auditState) *models.AuditEvent {
	body := models.JSONObject{}

	if len(bodyBytes) > 0 {
		var parsed map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
			body = redact(parsed, redactKeys)
		}
	}

	st.mu.Lock()
	for k, v := range st.detail {
		body[k] = v
	}
	resourceID := st.resourceID
	st.mu.Unlock()

	ev := &models.AuditEvent{
		Resource:      tag.Resource,
		Action:        tag.Action,
		RequestMethod: r.Method,
		RequestPath:   r.URL.Path,
		ResultStatus:  status,
		Body:          body,
	}

	if resourceID != "" {
		ev.ResourceID = &resourceID
	}

	if u, ok := GetOIDCUser(r); ok && u != nil {
		ev.ActorLabel = u.Email
		if ev.ActorLabel == "" {
			ev.ActorLabel = u.Sub
		}
	}

	if ou, ok := GetOrgUser(r); ok && ou != nil {
		ev.ActorOrgUserID = &ou.ID
		ev.OrganizationID = &ou.OrganizationID

		if ev.ActorLabel == "" {
			ev.ActorLabel = ou.Email
		}
	}

	return ev
}

// redact returns a copy of m with secret-keyed values replaced by "[redacted]",
// recursing into nested objects.
func redact(m map[string]interface{}, keys []string) models.JSONObject {
	out := models.JSONObject{}

	for k, v := range m {
		if matchesRedact(k, keys) {
			out[k] = "[redacted]"

			continue
		}

		if nested, ok := v.(map[string]interface{}); ok {
			out[k] = map[string]interface{}(redact(nested, keys))

			continue
		}

		out[k] = v
	}

	return out
}

func matchesRedact(key string, keys []string) bool {
	lower := strings.ToLower(key)

	for _, rk := range keys {
		if strings.Contains(lower, rk) {
			return true
		}
	}

	return false
}
