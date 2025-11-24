CREATE TABLE xi_feedbacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES xi_users(id),
    liked INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_xi_feedbacks_user_id ON xi_feedbacks(user_id);
CREATE INDEX idx_xi_feedbacks_created_at ON xi_feedbacks(created_at);