-- Добавляем поле is_banless в таблицу xi_users
ALTER TABLE xi_users ADD COLUMN IF NOT EXISTS is_banless BOOLEAN NOT NULL DEFAULT false;

-- Создаем таблицу xi_bans для хранения временных банов
CREATE TABLE IF NOT EXISTS xi_bans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES xi_users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    duration VARCHAR(50) NOT NULL,
    banned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    banned_where BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Создаем индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_xi_bans_user_id ON xi_bans(user_id);
CREATE INDEX IF NOT EXISTS idx_xi_bans_banned_at ON xi_bans(banned_at);
CREATE INDEX IF NOT EXISTS idx_xi_bans_banned_where ON xi_bans(banned_where);
