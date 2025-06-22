CREATE TABLE IF NOT EXISTS xi_pins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id BIGINT NOT NULL,
    user_id UUID NOT NULL REFERENCES xi_users(id),
    message TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT xi_pins_message_length CHECK (length(message) <= 1024)
);

CREATE INDEX IF NOT EXISTS idx_xi_pins_chat_id ON xi_pins(chat_id);
CREATE INDEX IF NOT EXISTS idx_xi_pins_user_id ON xi_pins(user_id);
CREATE INDEX IF NOT EXISTS idx_xi_pins_created_at ON xi_pins(created_at);
CREATE INDEX IF NOT EXISTS idx_xi_pins_chat_user ON xi_pins(chat_id, user_id); 