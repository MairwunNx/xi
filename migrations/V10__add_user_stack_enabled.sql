ALTER TABLE xi_users ADD COLUMN IF NOT EXISTS is_stack_enabled BOOLEAN NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS idx_xi_users_is_stack_enabled ON xi_users(is_stack_enabled); 