-- Normalize case-colliding emails so we can enforce case-insensitive uniqueness.
WITH duplicate_candidates AS (
    SELECT
        id,
        email,
        split_part(email, '@', 1) AS local_part,
        split_part(email, '@', 2) AS domain_part,
        row_number() OVER (PARTITION BY lower(email) ORDER BY id ASC) AS rn
    FROM users
)
UPDATE users AS u
SET email = lower(dc.local_part) || '.dup' || dc.id::text || '@' || lower(dc.domain_part)
FROM duplicate_candidates AS dc
WHERE u.id = dc.id
    AND dc.rn > 1;

-- Replace case-sensitive uniqueness with case-insensitive uniqueness.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
DROP INDEX IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower_unique ON users ((lower(email)));

-- Support common list/query patterns with composite indexes.
CREATE INDEX IF NOT EXISTS idx_hosts_org_last_logger_at ON hosts (organization_id, last_logger_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_campaigns_org_created_at ON campaigns (organization_id, created_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_campaign_targets_host_updated_campaign ON campaign_targets (host_id, updated_at DESC, campaign_id);
