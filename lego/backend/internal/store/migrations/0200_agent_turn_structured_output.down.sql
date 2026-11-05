ALTER TABLE agent_session_turns
    DROP COLUMN IF EXISTS result_final,
    DROP COLUMN IF EXISTS result,
    DROP COLUMN IF EXISTS output_required,
    DROP COLUMN IF EXISTS output_schema;
