-- Add nickname column for user-friendly passkey names
ALTER TABLE user_credentials ADD COLUMN nickname TEXT;
