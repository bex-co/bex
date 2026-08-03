-- ADR049 D1: new Blueprint rows use Render's canonical filename. Existing
-- rows retain their explicit stored path, including the legacy bex.yml alias.
ALTER TABLE blueprints
    ALTER COLUMN path SET DEFAULT 'render.yaml';
