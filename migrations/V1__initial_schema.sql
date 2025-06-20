CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS xi_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    telegram_user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    username VARCHAR(255),
    full_name VARCHAR(255),
    message_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    message_text BYTEA NOT NULL,
    is_aggressive BOOLEAN NOT NULL,
    is_xi_response BOOLEAN NOT NULL,
    CONSTRAINT valid_message_text CHECK (length(message_text::text) > 0)
);

CREATE INDEX IF NOT EXISTS idx_xi_messages_chat_id ON xi_messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_xi_messages_telegram_user_id ON xi_messages(telegram_user_id);
CREATE INDEX IF NOT EXISTS idx_xi_messages_chat_time ON xi_messages(chat_id, message_time DESC);

CREATE TABLE IF NOT EXISTS xi_chat_modes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id BIGINT NOT NULL,
    is_aggressive BOOLEAN NOT NULL DEFAULT false,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    changed_by_username VARCHAR(255),
    changed_by_telegram_id BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_xi_chat_modes_lookup ON xi_chat_modes(chat_id, changed_at DESC);

CREATE OR REPLACE FUNCTION encrypt_message(message TEXT, key TEXT) RETURNS BYTEA AS $$
BEGIN
    RETURN pgp_sym_encrypt(message, key, 'cipher-algo=aes256');
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT;

CREATE OR REPLACE FUNCTION decrypt_message(encrypted_message BYTEA, key TEXT) RETURNS TEXT AS $$
BEGIN
    RETURN pgp_sym_decrypt(encrypted_message, key)::text;
EXCEPTION
    WHEN OTHERS THEN
        RETURN '';
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT; 