-- w9/m343: agent automations tell their creator how runs went (ADR101 D7,
-- Devin parity). notify_email is the per-automation email setting, Devin's
-- Always / On failure / On success plus Never, defaulting to on_failure as
-- Devin's scheduled sessions do. notify_failure_streak counts consecutive
-- failed runs for the failure backoff (mail the 1st, 3rd, 9th, ...) and is
-- separate from consecutive_failures, which counts only failed creates for
-- the auto-pause. paused_at marks a platform pause (a pause with a reason),
-- and pause_notified_at the last pause its creator was told about.
ALTER TABLE agent_automations
    ADD COLUMN notify_email text NOT NULL DEFAULT 'on_failure'
        CHECK (notify_email IN ('always', 'on_failure', 'on_success', 'never')),
    ADD COLUMN notify_failure_streak integer NOT NULL DEFAULT 0 CHECK (notify_failure_streak >= 0),
    ADD COLUMN paused_at timestamptz,
    ADD COLUMN pause_notified_at timestamptz;

-- Pauses that predate this migration were never notified and are not now.
UPDATE agent_automations
SET paused_at = updated_at, pause_notified_at = now()
WHERE paused_reason IS NOT NULL;

-- The pause push reads recent pauses by paused_at; the pause notice claim
-- reads only the pauses not yet notified.
CREATE INDEX agent_automations_pause_notify_idx
    ON agent_automations (paused_at)
    WHERE paused_reason IS NOT NULL AND paused_at IS NOT NULL;
CREATE INDEX agent_automations_pause_unnotified_idx
    ON agent_automations (paused_at)
    WHERE paused_reason IS NOT NULL AND paused_at IS NOT NULL AND pause_notified_at IS NULL;

-- A run is claimed for its email decision once, when it is settled and, for a
-- started run, its session has ended. Overlap skips are never claimed.
ALTER TABLE agent_automation_runs ADD COLUMN notified_at timestamptz;

-- Runs settled before this migration are not mailed, except a started run
-- whose session is still working: it is mailed when that session ends.
UPDATE agent_automation_runs r
SET notified_at = now()
WHERE r.outcome <> 'claimed'
  AND NOT (r.outcome = 'started' AND EXISTS (
      SELECT 1 FROM agent_sessions s
      WHERE s.id = r.session_id AND s.phase NOT IN ('completed', 'failed', 'canceled')));

CREATE INDEX agent_automation_runs_notify_idx
    ON agent_automation_runs (settled_at, automation_id, fire_at)
    WHERE notified_at IS NULL AND outcome NOT IN ('claimed', 'skipped_overlap');

-- The auto-pause push (agent_automation_paused) opens the automation, a third
-- workspace-keyed resource family beside agent sessions.
ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_event_type_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_event_type_check
    CHECK (event_type IN (
        'deploy_started', 'deploy_succeeded', 'deploy_failed',
        'server_failed', 'server_available',
        'service_suspended', 'service_resumed',
        'cron_failed',
        'agent_pr_ready', 'agent_failed', 'agent_needs_decision',
        'agent_automation_paused',
        'postgres_unavailable', 'postgres_available',
        'key_value_unhealthy', 'key_value_available',
        'postgres_backup_failed', 'postgres_restore_failed', 'postgres_upgrade_failed'
    ));

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_resource_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_resource_check
    CHECK (
        (resource_kind = 'service' AND resource_id ~ '^srv-[0-9a-v]{20}$')
        OR (resource_kind = 'agentSession' AND resource_id ~ '^ags-[0-9a-v]{20}$')
        OR (resource_kind = 'agentAutomation' AND resource_id ~ '^aut-[0-9a-v]{20}$')
        OR (resource_kind = 'database' AND resource_id ~ '^dpg-[0-9a-v]{20}$')
        OR (resource_kind = 'keyValue' AND resource_id ~ '^red-[0-9a-v]{20}$')
    );

ALTER TABLE push_notifications DROP CONSTRAINT push_notifications_deep_link_check;
ALTER TABLE push_notifications ADD CONSTRAINT push_notifications_deep_link_check
    CHECK (
        (resource_kind = 'service' AND deep_link = '/services/' || resource_id)
        OR (resource_kind = 'agentSession' AND deep_link = '/sessions/' || resource_id)
        OR (resource_kind = 'agentAutomation' AND deep_link = '/automations/' || resource_id)
        OR (resource_kind = 'database' AND deep_link = '/databases/' || resource_id)
        OR (resource_kind = 'keyValue' AND deep_link = '/key-values/' || resource_id)
    );
