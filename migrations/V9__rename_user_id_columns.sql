ALTER TABLE xi_donations RENAME COLUMN user_id TO "user";
ALTER TABLE xi_pins RENAME COLUMN user_id TO "user";
DROP INDEX IF EXISTS idx_xi_donations_user_id;
CREATE INDEX IF NOT EXISTS idx_xi_donations_user ON xi_donations("user");

DROP INDEX IF EXISTS idx_xi_pins_user_id;
CREATE INDEX IF NOT EXISTS idx_xi_pins_user ON xi_pins("user");

DROP INDEX IF EXISTS idx_xi_pins_chat_user;
CREATE INDEX IF NOT EXISTS idx_xi_pins_chat_user ON xi_pins(chat_id, "user"); 