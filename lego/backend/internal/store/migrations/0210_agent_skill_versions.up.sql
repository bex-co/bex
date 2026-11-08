-- ADR099 D2: every save of a workspace skill keeps an immutable version
-- (content, generated message, author, time, short id). The row is written in
-- the save's own transaction, the newest 200 per skill are kept (trimmed by
-- the store), and the versions go with the skill.
CREATE TABLE agent_skill_versions (
    skill_id   text        NOT NULL REFERENCES agent_skills (id) ON DELETE CASCADE,
    version    int         NOT NULL CHECK (version > 0),
    content    text        NOT NULL,
    message    text        NOT NULL,
    created_by text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    short_id   text        NOT NULL CHECK (short_id ~ '^[0-9a-f]{7}$'),
    PRIMARY KEY (skill_id, version)
);

-- One YAML scalar the way the Go renderer writes it: plain when it is plainly
-- a word or phrase, a double-quoted JSON string otherwise.
CREATE FUNCTION pg_temp.skill_yaml_scalar(v text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN v ~ '^[A-Za-z][A-Za-z0-9 .,()/_-]*$' AND v !~* '^(true|false|null|yes|no|on|off|y|n)$' AND v !~ ' $'
            THEN v
        ELSE to_json(v)::text
    END
$$;

-- Backfill: each existing skill gets a version row for its current version.
-- Earlier saves were never kept. content is rendered here from the stored
-- fields; the API renders the current version from the live row.
INSERT INTO agent_skill_versions (skill_id, version, content, message, created_by, created_at, short_id)
SELECT s.id, s.version, s.content,
       CASE WHEN s.version = 1 THEN 'Add' ELSE 'Update' END || ' skills/' || s.name || '/SKILL.md',
       s.updated_by, s.updated_at,
       substr(encode(sha256(convert_to(s.id || '|' || s.version || '|' || s.content, 'UTF8')), 'hex'), 1, 7)
FROM (
    SELECT id, version, name, updated_by, updated_at,
           '---' || E'\n' ||
           'name: ' || pg_temp.skill_yaml_scalar(name) || E'\n' ||
           'description: ' || pg_temp.skill_yaml_scalar(description) || E'\n' ||
           CASE WHEN triggers = '{user}' THEN 'triggers: [user]' || E'\n' ELSE '' END ||
           extra || '---' || E'\n' || body AS content
    FROM agent_skills
) s;

DROP FUNCTION pg_temp.skill_yaml_scalar(text);
