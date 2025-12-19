DROP INDEX IF EXISTS idx_hosts_org_host_identifier;

ALTER TABLE hosts ADD CONSTRAINT hosts_host_identifier_key UNIQUE (host_identifier);
