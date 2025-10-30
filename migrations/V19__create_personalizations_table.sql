CREATE TABLE IF NOT EXISTS xi_personalizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES xi_users(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT xi_personalizations_prompt_length CHECK (length(prompt) >= 12 AND length(prompt) <= 1024),
    CONSTRAINT xi_personalizations_unique_user UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_xi_personalizations_user_id ON xi_personalizations(user_id);
CREATE INDEX IF NOT EXISTS idx_xi_personalizations_created_at ON xi_personalizations(created_at);
