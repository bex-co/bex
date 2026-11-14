-- w4/653: Render's workspace Overlapping Deploy Policy (Workspace Settings →
-- Overlapping Deploy Policy, render.com/docs/deploys). 'wait' lets the deploy
-- in progress finish, then runs the most recently triggered one (Render's
-- default for workspaces created on or after 2025-07-14, so every bex
-- workspace); 'override' cancels the deploy in progress and starts the new one.
ALTER TABLE tenants ADD COLUMN overlapping_deploy_policy text NOT NULL DEFAULT 'wait'
    CHECK (overlapping_deploy_policy IN ('wait', 'override'));
