ALTER TABLE xi_messages ADD COLUMN IF NOT EXISTS is_removed BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_xi_messages_is_removed ON xi_messages(is_removed) WHERE is_removed = true; 