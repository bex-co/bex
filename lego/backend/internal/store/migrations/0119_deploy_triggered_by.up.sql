-- w4/072: persist who opened a deploy so service events can project
-- details.triggeredByUser. Same subject form as audit_events.caller.
-- Empty for git auto-deploy / deploy-hook / system paths with no session;
-- session and API-key callers store core.Identity.Subject.
ALTER TABLE deploys
    ADD COLUMN triggered_by text NOT NULL DEFAULT '';
