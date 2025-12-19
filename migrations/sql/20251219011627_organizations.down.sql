ALTER TABLE hosts DROP COLUMN IF EXISTS organization_id;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organization_enroll_secrets;
DROP TABLE IF EXISTS organizations;
