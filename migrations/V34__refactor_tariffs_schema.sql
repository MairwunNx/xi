ALTER TABLE xi_tariffs
    DROP COLUMN IF EXISTS dialer_models,
    DROP COLUMN IF EXISTS dialer_reasoning_effort,
    DROP COLUMN IF EXISTS context_ttl_seconds,
    DROP COLUMN IF EXISTS context_max_messages,
    DROP COLUMN IF EXISTS context_max_tokens,
    DROP COLUMN IF EXISTS usage_vision_daily,
    DROP COLUMN IF EXISTS usage_vision_monthly,
    DROP COLUMN IF EXISTS usage_dialer_daily,
    DROP COLUMN IF EXISTS usage_dialer_monthly,
    DROP COLUMN IF EXISTS usage_whisper_daily,
    DROP COLUMN IF EXISTS usage_whisper_monthly;

ALTER TABLE xi_tariffs
    ADD COLUMN requests_per_day INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN requests_per_month INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN tokens_per_day BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN tokens_per_month BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN price INTEGER NOT NULL DEFAULT 0;
