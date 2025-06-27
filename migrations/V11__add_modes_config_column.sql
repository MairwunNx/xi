ALTER TABLE xi_modes ADD COLUMN config json;

UPDATE xi_modes 
SET config = json_object(
    'prompt', prompt,
    'params', json_object()
)
WHERE config IS NULL;

ALTER TABLE xi_modes MODIFY COLUMN config json NOT NULL;