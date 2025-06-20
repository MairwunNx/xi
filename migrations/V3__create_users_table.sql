CREATE TYPE user_right AS ENUM ('switch_mode', 'edit_mode', 'manage_users');

CREATE TABLE IF NOT EXISTS xi_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL UNIQUE,
    username VARCHAR(255),
    fullname VARCHAR(255),
    rights user_right[] NOT NULL DEFAULT ARRAY[]::user_right[],
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_stack_allowed BOOLEAN NOT NULL DEFAULT false,
    window_limit BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_xi_users_user_id ON xi_users(user_id);
CREATE INDEX IF NOT EXISTS idx_xi_users_username ON xi_users(username);
CREATE INDEX IF NOT EXISTS idx_xi_users_is_active ON xi_users(is_active);

ALTER TABLE xi_messages DROP COLUMN IF EXISTS telegram_user_id;
ALTER TABLE xi_messages DROP COLUMN IF EXISTS username;
ALTER TABLE xi_messages DROP COLUMN IF EXISTS full_name;
ALTER TABLE xi_messages ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES xi_users(id);

CREATE INDEX IF NOT EXISTS idx_xi_messages_user_id ON xi_messages(user_id);

INSERT INTO xi_users (
    user_id, 
    username,
    fullname,
    rights, 
    is_active, 
    is_stack_allowed, 
    window_limit
) VALUES (
    362695653, 
    'mairwunnx',
    'Pavel Erokhin',
    ARRAY['switch_mode', 'edit_mode', 'manage_users']::user_right[],
    true,
    true,
    30000
) ON CONFLICT (user_id) DO NOTHING; 