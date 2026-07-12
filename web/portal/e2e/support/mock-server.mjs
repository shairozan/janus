// Hermetic mock of the license-server API for the portal e2e suite.
//
// The portal's BFF (lib/api.ts) makes server-side calls to LICENSE_SERVER_URL;
// pointing that at this process lets Playwright drive the real UI end-to-end
// without a Go backend, a database, or any external system (Cognito/Stripe/
// Mailgun) — the IQ/OQ "mock externals" boundary. State lives in memory and is
// reset between tests via POST /__reset, so specs stay deterministic.
//
// Run with plain node (no TS loader): `node e2e/support/mock-server.mjs`.
import { createHash } from "node:crypto";
import { createServer } from "node:http";

const PORT = Number(process.env.MOCK_PORT ?? 4319);
const ORG_ID = 1;

// ── seed / state ──────────────────────────────────────────────────────────────

function freshState() {
  return {
    keys: { active: null, history: [] },
    license: null, // the issued license JWT string, or null
    meRequests: [], // the caller's own license requests
    adminRequests: [
      reqRow(101, "pending"),
      reqRow(102, "pending"),
    ],
    rules: [],
    sso: null,
    proposals: [],
    agreementSeats: 10,
    // Janus-staff: whether the caller's /me reports is_staff, plus the JWKS
    // signing keys the staff Admin screen manages. Seeded with one active master
    // key so the list renders content before any rotation.
    staff: false,
    signingKeys: [
      {
        key_id: "key-seed",
        organization_id: null,
        public_key_pem: "-----BEGIN PUBLIC KEY-----\nSEED\n-----END PUBLIC KEY-----",
        active: true,
        created_at: "2026-01-01T00:00:00Z",
        expires_at: "2027-01-01T00:00:00Z",
      },
    ],
    // Staff agreements catalog (GET /api/v1/agreements/list); POST appends here.
    staffAgreements: [
      {
        id: 1,
        organization_id: ORG_ID,
        tier: "pro",
        max_seats: 10,
        start_date: "2026-01-01T00:00:00Z",
        end_date: "2027-01-01T00:00:00Z",
      },
    ],
    seq: 1000,
  };
}

let state = freshState();

function nextID() {
  state.seq += 1;

  return state.seq;
}

function reqRow(id, status) {
  return {
    id,
    organization_id: ORG_ID,
    org_user_id: 50 + id,
    agreement_id: 1,
    status,
    created_at: "2026-05-01T00:00:00Z",
  };
}

function fingerprint(pem) {
  const hex = createHash("sha256").update(pem).digest("hex").slice(0, 32);

  return (hex.match(/.{2}/g) ?? []).join(":");
}

// Simulate the server-side replace-key transaction: rotate the active key into
// history and (re)issue a license bound to the new key.
function setKey(pem, title) {
  const key = {
    id: nextID(),
    org_user_id: 1,
    fingerprint: fingerprint(pem + state.seq),
    title: title || undefined,
    created_at: "2026-06-01T00:00:00Z",
  };

  if (state.keys.active) {
    state.keys.history.unshift({ ...state.keys.active, revoked_at: "2026-06-01T00:00:00Z" });
  }

  state.keys.active = key;
  // A license is (re)issued on key set — the value changes so a replacement is
  // observably a new license.
  state.license = `eyJ.e2e-license-${key.id}.sig`;

  return key;
}

const profile = () => ({
  id: 1,
  organization_id: ORG_ID,
  email: "admin@acme.test",
  role: "customer_admin",
  has_active_key: state.keys.active !== null,
  has_license: state.license !== null,
  is_staff: state.staff,
});

// A proposed offer with a single per-seat line item (the mock plays "Janus
// staff" and prices the request instantly, so the negotiation UI is reachable
// in one journey).
function proposeFor(seats, licenseModel, tier) {
  const unit = 120000; // $1,200.00 / seat / yr
  const amount = unit * seats;

  return {
    id: nextID(),
    organization_id: ORG_ID,
    requested_by_user_id: 1,
    status: "proposed",
    tier: tier ?? null,
    seats,
    license_model: licenseModel,
    validity_days: 365,
    total_amount_cents: amount,
    created_at: "2026-06-02T00:00:00Z",
    line_items: [
      {
        id: nextID(),
        proposal_id: 0,
        price_id: 7,
        quantity: seats,
        amount_cents: amount,
        price: {
          id: 7,
          nickname: "Pro per-seat (annual)",
          unit_amount_cents: unit,
          currency: "usd",
          interval: "year",
          kind: "per_seat",
        },
      },
    ],
  };
}

function agreementsWithUsage() {
  return [
    {
      id: 1,
      organization_id: ORG_ID,
      seats: state.agreementSeats,
      used_seats: 4,
      license_model: "subscription",
      tier: "pro",
      price_per_seat_cents: 120000,
    },
  ];
}

// ── routing ───────────────────────────────────────────────────────────────────

function send(res, status, body) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(body === undefined ? "" : JSON.stringify(body));
}

const ORG = `/api/v1/orgs/${ORG_ID}`;

function route(method, path, body) {
  // control plane
  if (path === "/__health") {
    return [200, { ok: true }];
  }

  if (path === "/__reset" && method === "POST") {
    state = freshState();

    return [200, { ok: true }];
  }

  // Flip the caller to Janus-staff (so /me reports is_staff → the Admin UI shows).
  if (path === "/__staff" && method === "POST") {
    state.staff = true;

    return [200, { ok: true }];
  }

  // ── end-user /me ──
  if (path === "/api/v1/me" && method === "GET") {
    return [200, profile()];
  }

  if (path === "/api/v1/me/key" && method === "GET") {
    return [200, state.keys];
  }

  if (path === "/api/v1/me/key" && method === "PUT") {
    return [200, setKey(body?.public_key_pem ?? "", body?.title ?? "")];
  }

  if (path === "/api/v1/me/license-requests" && method === "GET") {
    return [200, state.meRequests];
  }

  if (path === "/api/v1/me/license-requests" && method === "POST") {
    const r = reqRow(nextID(), "pending");
    state.meRequests.unshift(r);

    return [201, r];
  }

  if (path === "/api/v1/me/license" && method === "GET") {
    return state.license ? [200, { license: state.license }] : [404, { error: "no license" }];
  }

  // ── admin members ──
  if (path === `${ORG}/members` && method === "GET") {
    return [200, []];
  }

  // ── admin license requests ──
  if (path === `${ORG}/license-requests` && method === "GET") {
    return [200, state.adminRequests];
  }

  let m = path.match(/^\/api\/v1\/orgs\/1\/license-requests\/(\d+)\/(approve|reject)$/);
  if (m && method === "POST") {
    const r = state.adminRequests.find((x) => x.id === Number(m[1]));
    if (!r) {
      return [404, { error: "request not found" }];
    }

    r.status = m[2] === "approve" ? "approved" : "rejected";
    if (m[2] === "reject" && body?.reason) {
      r.reason = body.reason;
    }

    return [200, r];
  }

  // ── auto-acceptance rules ──
  if (path === `${ORG}/rules` && method === "GET") {
    return [200, state.rules];
  }

  if (path === `${ORG}/rules` && method === "POST") {
    const rule = {
      id: nextID(),
      organization_id: ORG_ID,
      rule_type: body?.rule_type,
      match_domain: body?.match_domain ?? null,
      max_auto_seats: body?.max_auto_seats ?? null,
      enabled: true,
      created_by_user_id: 1,
      created_at: "2026-06-02T00:00:00Z",
    };
    state.rules.push(rule);

    return [201, rule];
  }

  m = path.match(/^\/api\/v1\/orgs\/1\/rules\/(\d+)$/);
  if (m && method === "PUT") {
    const rule = state.rules.find((x) => x.id === Number(m[1]));
    if (!rule) {
      return [404, { error: "rule not found" }];
    }

    rule.enabled = Boolean(body?.enabled);

    return [200, rule];
  }

  if (m && method === "DELETE") {
    state.rules = state.rules.filter((x) => x.id !== Number(m[1]));

    return [200, { status: "deleted" }];
  }

  // ── activity ──
  if (path === `${ORG}/activity` && method === "GET") {
    return [200, []];
  }

  // ── SSO (create-once + read-only) ──
  if (path === `${ORG}/sso` && method === "GET") {
    return state.sso ? [200, state.sso] : [404, { error: "no sso configuration" }];
  }

  if (path === `${ORG}/sso` && method === "POST") {
    const name = body?.provider_name ?? "idp";
    state.sso = {
      id: nextID(),
      organization_id: ORG_ID,
      provider_name: name,
      user_pool_id: "us-east-2_e2epool",
      oidc_issuer: body?.oidc_issuer ?? "",
      oidc_client_id: body?.client_id ?? "",
      scopes: body?.scopes ?? "openid email profile",
      attribute_mapping: body?.attribute_map ?? { email: "email" },
      cognito_status: "active",
      login_url: `https://auth.januspk.com/login?identity_provider=${encodeURIComponent(name)}`,
      has_client_secret: Boolean(body?.client_secret),
    };

    return [201, state.sso];
  }

  // ── seats ──
  if (path === `${ORG}/agreements` && method === "GET") {
    return [200, agreementsWithUsage()];
  }

  m = path.match(/^\/api\/v1\/orgs\/1\/agreements\/(\d+)\/seats\/preview$/);
  if (m && method === "GET") {
    const add = Number(new URL("http://x" + (this?.search ?? "")).searchParams.get("add") ?? 0);

    return [200, seatPreview(add)];
  }

  m = path.match(/^\/api\/v1\/orgs\/1\/agreements\/(\d+)\/seats$/);
  if (m && method === "POST") {
    state.agreementSeats += Number(body?.add ?? 0);

    return [200, agreementsWithUsage()[0]];
  }

  // ── proposals / onboarding ──
  if (path === `${ORG}/proposals` && method === "GET") {
    return [200, state.proposals];
  }

  if (path === `${ORG}/proposals` && method === "POST") {
    const p = proposeFor(Number(body?.seats ?? 1), body?.license_model ?? "subscription", body?.tier);
    p.line_items[0].proposal_id = p.id;
    state.proposals = [p];

    return [201, p];
  }

  m = path.match(/^\/api\/v1\/orgs\/1\/proposals\/(\d+)\/(counter|accept|pay)$/);
  if (m && method === "POST") {
    const p = state.proposals.find((x) => x.id === Number(m[1]));
    if (!p) {
      return [404, { error: "proposal not found" }];
    }

    if (m[2] === "accept") {
      p.status = "accepted";
    } else if (m[2] === "pay") {
      p.status = "fulfilled";
      p.resulting_agreement_id = 1;
    } else {
      p.seats = Number(body?.seats ?? p.seats);
      p.status = "countered";
    }

    return [200, p];
  }

  m = path.match(/^\/api\/v1\/orgs\/1\/proposals\/(\d+)$/);
  if (m && method === "GET") {
    const p = state.proposals.find((x) => x.id === Number(m[1]));

    return p ? [200, p] : [404, { error: "proposal not found" }];
  }

  // ── Janus-staff: signing keys ──
  if (path === "/api/v1/keys" && method === "GET") {
    const orgFilter = new URL("http://x" + (this?.search ?? "")).searchParams.get("organization_id");
    const keys = orgFilter
      ? state.signingKeys.filter((k) => String(k.organization_id) === orgFilter)
      : state.signingKeys;

    return [200, keys];
  }

  if (path === "/api/v1/keys/rotate" && method === "POST") {
    const orgID = body?.organization_id ?? null;
    const days = body?.expires_in_days ?? 365;
    // Retire the prior active key for this scope.
    const prior = state.signingKeys.find((k) => k.active && k.organization_id === orgID);
    if (prior) prior.active = false;

    const newKey = {
      key_id: `key-${nextID()}`,
      organization_id: orgID,
      public_key_pem: `-----BEGIN PUBLIC KEY-----\nROTATED${state.seq}\n-----END PUBLIC KEY-----`,
      active: true,
      created_at: "2026-06-02T00:00:00Z",
      expires_at: "2027-06-02T00:00:00Z",
    };
    state.signingKeys.unshift(newKey);

    return [
      200,
      {
        new_key_id: newKey.key_id,
        old_key_id: prior?.key_id ?? "",
        expires_at: newKey.expires_at,
        public_key: newKey.public_key_pem,
        _days: days, // echoed only so the param is observably consumed
      },
    ];
  }

  // ── Janus-staff: org + agreement pickers ──
  if (path === "/api/v1/organizations/list" && method === "GET") {
    return [200, [{ id: ORG_ID, name: "Acme", customer_id: "CUST-ACME" }]];
  }

  if (path === "/api/v1/agreements/list" && method === "GET") {
    return [200, state.staffAgreements];
  }

  // Staff: list license tiers (the agreement-create catalog).
  if (path === "/api/v1/licenses" && method === "GET") {
    return [
      200,
      [
        { id: 1, name: "Professional License", tier: "professional", features: ["basic", "local", "grid"], msrp_cents: 5000 },
        { id: 2, name: "Enterprise License", tier: "enterprise", features: ["basic", "local", "grid", "audit"], msrp_cents: 10000 },
      ],
    ];
  }

  // Staff: define an agreement directly (POST /api/v1/agreements).
  if (path === "/api/v1/agreements" && method === "POST") {
    if (!body?.organization_id) return [400, { error: "organization_id is required" }];
    if (!body?.tier) return [400, { error: "tier is required" }];
    if (!body?.max_seats || body.max_seats <= 0) return [400, { error: "max_seats must be > 0" }];
    if (!body?.start_date) return [400, { error: "start_date is required" }];

    const agreement = {
      id: nextID(),
      organization_id: body.organization_id,
      tier: body.tier,
      max_seats: body.max_seats,
      start_date: `${body.start_date}T00:00:00Z`,
      end_date: body.end_date ? `${body.end_date}T00:00:00Z` : null,
    };
    state.staffAgreements.push(agreement);

    return [201, agreement];
  }

  // ── Janus-staff: issue a license ──
  if (path === "/api/v1/tokens" && method === "POST") {
    if (!body?.agreement_id) return [400, { error: "agreement_id is required" }];
    if (!body?.user_email) return [400, { error: "user_email is required" }];

    return [200, { token: `eyJ.e2e-issued-${nextID()}.sig`, expires_at: "2027-01-01T00:00:00Z" }];
  }

  return [404, { error: `unmocked ${method} ${path}` }];
}

function seatPreview(add) {
  const unit = 120000;

  return {
    current_seats: 10,
    add_seats: add,
    new_seats: 10 + add,
    prorated_charge_cents: Math.round(unit * add * 0.5),
    recurring_delta_cents: unit * add,
  };
}

const server = createServer((req, res) => {
  const url = new URL(req.url ?? "/", "http://localhost");
  const chunks = [];
  req.on("data", (c) => chunks.push(c));
  req.on("end", () => {
    let body;
    if (chunks.length) {
      try {
        body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      } catch {
        body = undefined;
      }
    }

    let result;
    try {
      // `search` is threaded via `this` for the one route that needs the query.
      result = route.call({ search: url.search }, req.method ?? "GET", url.pathname, body);
    } catch (err) {
      result = [500, { error: String(err) }];
    }

    send(res, result[0], result[1]);
  });
});

server.listen(PORT, () => {
  // eslint-disable-next-line no-console
  console.log(`[mock-license-server] listening on http://localhost:${PORT}`);
});
