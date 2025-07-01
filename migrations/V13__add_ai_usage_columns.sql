ALTER TABLE xi_messages 
ADD COLUMN cost DECIMAL(10,6) DEFAULT 0.0,
ADD COLUMN tokens INTEGER DEFAULT 0;

CREATE INDEX idx_xi_messages_cost ON xi_messages(cost);
CREATE INDEX idx_xi_messages_tokens ON xi_messages(tokens);
CREATE INDEX idx_xi_messages_user_cost ON xi_messages(user_id, cost);
CREATE INDEX idx_xi_messages_user_tokens ON xi_messages(user_id, tokens); 