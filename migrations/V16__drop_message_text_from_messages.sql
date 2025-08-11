ALTER TABLE xi_messages DROP COLUMN message_text;
DROP FUNCTION IF EXISTS encrypt_message(TEXT, TEXT);
DROP FUNCTION IF EXISTS decrypt_message(BYTEA, TEXT);
ALTER TABLE xi_messages DROP COLUMN is_aggressive;
