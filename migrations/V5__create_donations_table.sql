CREATE TABLE IF NOT EXISTS xi_donations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES xi_users(id),
    sum DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_xi_donations_user_id ON xi_donations(user_id);
CREATE INDEX IF NOT EXISTS idx_xi_donations_created_at ON xi_donations(created_at); 