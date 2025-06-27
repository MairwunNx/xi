ALTER TABLE xi_modes ADD COLUMN final BOOLEAN NOT NULL DEFAULT false;

UPDATE xi_modes SET final = false WHERE final IS NULL;