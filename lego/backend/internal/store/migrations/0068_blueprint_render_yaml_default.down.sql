-- Roll back only the database default; existing explicit Blueprint paths are
-- intentionally never rewritten by this migration.
ALTER TABLE blueprints
    ALTER COLUMN path SET DEFAULT 'bex.yml';
