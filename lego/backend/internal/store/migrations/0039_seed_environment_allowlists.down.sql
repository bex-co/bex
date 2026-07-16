-- Reverse of the w4/m28 seed: rows holding exactly the migration-seed pair
-- return to empty (pre-m28 empty-means-open semantics).
UPDATE environments
SET ip_allow_list = '[]'::jsonb
WHERE ip_allow_list = '[{"cidrBlock":"0.0.0.0/0","description":"Allow all (migration seed)"},{"cidrBlock":"::/0","description":"Allow all (migration seed)"}]'::jsonb;
