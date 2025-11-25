ALTER TABLE xi_feedbacks ADD COLUMN kind VARCHAR(20) NOT NULL DEFAULT 'dialer';

CREATE INDEX idx_xi_feedbacks_kind ON xi_feedbacks(kind);