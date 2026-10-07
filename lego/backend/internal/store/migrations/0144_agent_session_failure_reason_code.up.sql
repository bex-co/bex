-- w5/m132: a failed agent session records why as a sentence (failure_reason)
-- and, when a client acts on the cause, a stable code beside it. The
-- dashboard's upgrade callout decides by the code, never by the wording.
ALTER TABLE agent_sessions ADD COLUMN IF NOT EXISTS failure_reason_code text NOT NULL DEFAULT '';

-- Sessions refused for sandbox capacity before this migration. Since w5/m80
-- (9724686a7) the sentence is written to failure_reason; before it, to status.
UPDATE agent_sessions SET failure_reason_code = 'SANDBOX_CAPACITY_LIMIT'
WHERE 'sandbox capacity reached' IN (failure_reason, status);
