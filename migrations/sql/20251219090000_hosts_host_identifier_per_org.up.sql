-- Allow the same host_identifier across organizations.
-- This prevents cross-organization host takeover during enrollment.

ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_host_identifier_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_hosts_org_host_identifier ON hosts(organization_id, host_identifier);
