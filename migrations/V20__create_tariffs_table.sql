CREATE TABLE xi_tariffs (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    dialer_models JSONB NOT NULL,
    dialer_reasoning_effort VARCHAR(50) NOT NULL,
    
    vision_primary_model VARCHAR(100) NOT NULL,
    vision_fallback_models TEXT[] NOT NULL DEFAULT '{}',
    
    context_ttl_seconds INTEGER NOT NULL,
    context_max_messages INTEGER NOT NULL,
    context_max_tokens INTEGER NOT NULL,
    
    usage_vision_daily INTEGER NOT NULL,
    usage_vision_monthly INTEGER NOT NULL,
    usage_dialer_daily INTEGER NOT NULL,
    usage_dialer_monthly INTEGER NOT NULL,
    usage_whisper_daily INTEGER NOT NULL,
    usage_whisper_monthly INTEGER NOT NULL,
    
    spending_daily_limit DECIMAL(10, 2) NOT NULL,
    spending_monthly_limit DECIMAL(10, 2) NOT NULL
);

CREATE INDEX idx_xi_tariffs_key_created_at ON xi_tariffs (key, created_at DESC);