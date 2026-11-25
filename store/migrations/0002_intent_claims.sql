ALTER TABLE exploration_nodes
    ADD COLUMN IF NOT EXISTS claim_expires_at TIMESTAMPTZ;
