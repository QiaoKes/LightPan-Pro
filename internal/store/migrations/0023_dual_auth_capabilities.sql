ALTER TABLE account_auth_states ADD COLUMN capabilities_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE account_auth_states ADD COLUMN credential_generation INTEGER NOT NULL DEFAULT 0;
