-- Add nickname column for user-friendly passkey names
ALTER TABLE user_credentials ADD COLUMN IF NOT EXISTS nickname TEXT;
