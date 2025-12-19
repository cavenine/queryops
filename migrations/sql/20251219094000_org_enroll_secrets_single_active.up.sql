-- Enforce at most one active enrollment secret per organization.

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_enroll_secrets_one_active
ON organization_enroll_secrets (organization_id)
WHERE active;
