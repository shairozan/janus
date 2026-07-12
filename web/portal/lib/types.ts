// Portal-facing types mirroring the license-server JSON (the Go models' json tags).

export const ROLE_ADMIN = "customer_admin";
export const ROLE_MEMBER = "member";

export interface Profile {
  id: number;
  organization_id: number;
  email: string;
  role: string;
  has_active_key: boolean;
  has_license: boolean;
  // True when the caller's email domain is in the Janus-staff allowlist. Drives
  // the staff "Admin" area only — the staff endpoints stay enforced server-side.
  is_staff: boolean;
}

export interface OrgUser {
  id: number;
  organization_id: number;
  cognito_sub: string;
  email: string;
  role: string;
  source: string;
  created_at: string;
  deactivated_at?: string | null;
}

export interface PublicKey {
  id: number;
  org_user_id: number;
  fingerprint: string;
  title?: string;
  created_at: string;
  revoked_at?: string | null;
}

export interface KeysResponse {
  active: PublicKey | null;
  history: PublicKey[];
}

export interface ReplaceKeyInput {
  publicKeyPem: string;
  title: string;
}

export interface LicenseRequest {
  id: number;
  organization_id: number;
  org_user_id: number;
  agreement_id: number;
  status: string; // pending | approved | rejected | failed
  decision?: string | null;
  reason?: string | null;
  issued_token_id?: number | null;
  created_at: string;
}

// ── auto-acceptance rules ─────────────────────────────────────────────────────

export const RULE_EMAIL_DOMAIN = "email_domain";
export const RULE_SEAT_THRESHOLD = "seat_threshold";

export interface AutoAcceptanceRule {
  id: number;
  organization_id: number;
  agreement_id?: number | null;
  rule_type: string; // email_domain | seat_threshold
  match_domain?: string | null;
  max_auto_seats?: number | null;
  enabled: boolean;
  created_by_user_id: number;
  created_at: string;
  deactivated_at?: string | null;
}

export interface CreateRuleInput {
  rule_type: string;
  match_domain?: string;
  max_auto_seats?: number;
  agreement_id?: number;
}

// ── seat management ───────────────────────────────────────────────────────────

export interface AgreementUsage {
  id: number;
  organization_id: number;
  seats: number;
  used_seats: number;
  license_model: string;
  tier?: string | null;
  price_per_seat_cents: number;
  start_date?: string | null;
  end_date?: string | null;
}

export interface SeatPreview {
  current_seats: number;
  add_seats: number;
  new_seats: number;
  prorated_charge_cents: number;
  recurring_delta_cents: number;
}

// ── SSO ───────────────────────────────────────────────────────────────────────

export interface SSOConfig {
  id: number;
  organization_id: number;
  provider_name: string;
  user_pool_id: string;
  oidc_issuer: string;
  oidc_client_id: string;
  scopes: string;
  attribute_mapping?: Record<string, string> | null;
  cognito_status: string; // pending | active | failed
  login_url: string;
  has_client_secret: boolean;
}

export interface SSOSetupInput {
  provider_name: string;
  oidc_issuer: string;
  client_id: string;
  client_secret: string;
  scopes: string;
  attribute_map: Record<string, string>;
}

// ── proposals / onboarding ────────────────────────────────────────────────────

export const PROPOSAL_REQUESTED = "requested";
export const PROPOSAL_PROPOSED = "proposed";
export const PROPOSAL_COUNTERED = "countered";
export const PROPOSAL_ACCEPTED = "accepted";
export const PROPOSAL_PAID = "paid";
export const PROPOSAL_FULFILLED = "fulfilled";
export const PROPOSAL_REJECTED = "rejected";
export const PROPOSAL_EXPIRED = "expired";

export interface StripePrice {
  id: number;
  nickname: string;
  unit_amount_cents: number;
  currency: string;
  interval: string; // month | year | one_time
  kind: string; // per_seat | flat | concurrent
}

export interface ProposalLineItem {
  id: number;
  proposal_id: number;
  price_id: number;
  quantity: number;
  amount_cents: number;
  price?: StripePrice | null;
}

export interface AgreementProposal {
  id: number;
  organization_id: number;
  requested_by_user_id: number;
  status: string;
  tier?: string | null;
  seats: number;
  license_model: string;
  validity_days: number;
  total_amount_cents: number;
  message?: string | null;
  parent_proposal_id?: number | null;
  resulting_agreement_id?: number | null;
  created_at: string;
  line_items?: ProposalLineItem[] | null;
}

export interface RequestProposalInput {
  tier?: string;
  seats: number;
  license_model: string;
  validity_days?: number;
  message?: string;
}

// ── staff: signing keys & license issuance ───────────────────────────────────
// These mirror the Janus-staff license-server endpoints surfaced in the portal's
// "Admin" area (gated by Profile.is_staff). The API enforces staff access itself.

export interface SigningKey {
  key_id: string;
  organization_id?: number | null; // null/absent → the master (global) key
  public_key_pem: string;
  active: boolean;
  created_at: string;
  expires_at: string;
  revoked_at?: string | null;
}

export interface RotateKeyInput {
  organization_id?: number; // omit for the master key
  expires_in_days?: number; // defaults to 365 server-side
}

export interface RotateKeyResult {
  new_key_id: string;
  old_key_id?: string;
  expires_at: string;
  public_key: string; // PEM
}

export interface IssueLicenseInput {
  agreement_id: number;
  user_email: string;
  duration_seconds?: number; // omit → derives from the agreement term
  signing_public_key?: string; // optional client RSA public key (PEM) embedded for run-log signing
}

export interface IssuedLicense {
  token: string; // the signed license JWT
  expires_at: string;
}

// A license tier/product (the catalog the agreement-create form picks from).
export interface LicenseTier {
  id: number;
  name: string;
  tier: string;
  features: string[];
  msrp_cents: number;
}

export interface CreateAgreementInput {
  organization_id: number;
  tier: string;
  max_seats: number;
  start_date: string; // YYYY-MM-DD
  end_date?: string; // YYYY-MM-DD (optional)
  features?: string[];
  cost_per_month?: number; // dollars
}

// Minimal staff-side org/agreement summaries used by the keys/license pickers.
export interface OrgSummary {
  id: number;
  name: string;
  customer_id: string;
  deactivated_at?: string | null;
}

export interface AgreementSummary {
  id: number;
  organization_id: number;
  tier: string;
  max_seats: number;
  start_date: string;
  end_date?: string | null;
  deactivated_at?: string | null;
}

// ── activity log ──────────────────────────────────────────────────────────────

export interface AuditEvent {
  id: string;
  occurred_at: string;
  organization_id?: number | null;
  actor_org_user_id?: number | null;
  actor_label: string;
  resource: string;
  action: string;
  resource_id?: string | null;
  request_method: string;
  request_path: string;
  result_status: number;
}
