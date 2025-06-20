DROP TABLE IF EXISTS xi_chat_modes;

CREATE TABLE IF NOT EXISTS xi_modes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id BIGINT NOT NULL,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    prompt TEXT NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES xi_users(id)
);

CREATE INDEX IF NOT EXISTS idx_xi_modes_chat_id ON xi_modes(chat_id);
CREATE INDEX IF NOT EXISTS idx_xi_modes_type ON xi_modes(type);
CREATE INDEX IF NOT EXISTS idx_xi_modes_is_enabled ON xi_modes(is_enabled) WHERE is_enabled = true;
CREATE UNIQUE INDEX IF NOT EXISTS idx_xi_modes_chat_type ON xi_modes(chat_id, type) WHERE chat_id != 0;

CREATE TABLE IF NOT EXISTS xi_selected_modes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id BIGINT NOT NULL,
    mode_id UUID NOT NULL REFERENCES xi_modes(id),
    switched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    switched_by UUID NOT NULL REFERENCES xi_users(id)
);

CREATE INDEX IF NOT EXISTS idx_xi_selected_modes_chat_id ON xi_selected_modes(chat_id);
CREATE INDEX IF NOT EXISTS idx_xi_selected_modes_switched_at ON xi_selected_modes(switched_at);

INSERT INTO xi_modes (
    id,
    chat_id,
    type,
    name,
    prompt,
    is_enabled,
    created_by
) VALUES (
    '00000000-0000-0000-0000-000000000000',
    0,
    'normal',
    'обычный 😇',
    '{{magic_prompt}}',
    true,
    NULL
) ON CONFLICT (id) DO NOTHING; 
