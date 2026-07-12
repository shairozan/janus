-- Integration-test seed: one organization + one customer-admin user whose
-- cognito_sub matches the subject the Playwright login submits to the mock OIDC
-- server (test/integration/specs/me.spec.ts → SUBJECT). Idempotent so the seed
-- container can re-run safely.

INSERT INTO organizations (name, customer_id, created_at)
VALUES ('Integration Org', 'CUST-INT-E2E', now())
ON CONFLICT (customer_id) DO NOTHING;

INSERT INTO org_users (organization_id, cognito_sub, email, role, source, created_at, updated_at)
SELECT o.id, 'integration-user', 'integration@example.test', 'customer_admin', 'sso', now(), now()
FROM organizations o
WHERE o.customer_id = 'CUST-INT-E2E'
ON CONFLICT (cognito_sub) DO NOTHING;
