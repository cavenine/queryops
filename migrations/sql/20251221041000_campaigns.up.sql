CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT,
    description TEXT,
    query TEXT NOT NULL,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'pending',
    target_count INTEGER NOT NULL DEFAULT 0,
    result_count INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT campaigns_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_campaigns_organization_id ON campaigns(organization_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_created_at ON campaigns(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_campaigns_status_active ON campaigns(status) WHERE status IN ('pending', 'running');

CREATE TABLE IF NOT EXISTS campaign_targets (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    sent_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    results JSONB,
    error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, host_id),
    CONSTRAINT campaign_targets_status_check CHECK (status IN ('pending', 'sent', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_campaign_targets_host_id_status ON campaign_targets(host_id, status);
CREATE INDEX IF NOT EXISTS idx_campaign_targets_campaign_updated_at ON campaign_targets(campaign_id, updated_at DESC);
