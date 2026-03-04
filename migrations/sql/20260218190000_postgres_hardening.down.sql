DROP INDEX IF EXISTS idx_campaign_targets_host_updated_campaign;
DROP INDEX IF EXISTS idx_campaigns_org_created_at;
DROP INDEX IF EXISTS idx_hosts_org_last_logger_at;

DROP INDEX IF EXISTS idx_users_email_lower_unique;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_email_key'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
    END IF;
END $$;
