-- w1/m647: an admin of a workspace that requires two-factor authentication can
-- email its members who have not enabled it a reminder, at most one batch per
-- workspace per UTC day, as Render's "Remind limit has been reached!" allows.
-- Turning the requirement on sends the day's batch itself. sent_on is the UTC
-- day of the last batch; a batch claims the day by moving it forward, so two
-- admins sending at once send one batch.
CREATE TABLE workspace_two_factor_reminders (
    workspace_id text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    sent_on      date NOT NULL
);
