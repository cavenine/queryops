-- WebAuthn credentials table for passkey authentication
-- Users can have multiple passkeys (e.g., laptop TouchID, phone FaceID, YubiKey)
CREATE TABLE user_credentials (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Core credential data from WebAuthn
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    attestation_type TEXT NOT NULL,
    
    -- Transports hint which methods can be used (usb, nfc, ble, internal, hybrid)
    transports TEXT[],
    
    -- Credential flags
    flag_user_present BOOLEAN NOT NULL DEFAULT false,
    flag_user_verified BOOLEAN NOT NULL DEFAULT false,
    flag_backup_eligible BOOLEAN NOT NULL DEFAULT false,
    flag_backup_state BOOLEAN NOT NULL DEFAULT false,
    
    -- Authenticator info
    aaguid BYTEA,
    sign_count INTEGER NOT NULL DEFAULT 0,
    clone_warning BOOLEAN NOT NULL DEFAULT false,
    
    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_user_credentials_user_id ON user_credentials(user_id);
CREATE INDEX idx_user_credentials_credential_id ON user_credentials(credential_id);
