CREATE TABLE xi_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES xi_users(id) ON DELETE CASCADE,
    cost DECIMAL(10, 6) NOT NULL,
    tokens INT NOT NULL,
    chat_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_xi_usage_user_id ON xi_usage(user_id);
CREATE INDEX idx_xi_usage_created_at ON xi_usage(created_at);
CREATE INDEX idx_xi_usage_chat_id ON xi_usage(chat_id); 