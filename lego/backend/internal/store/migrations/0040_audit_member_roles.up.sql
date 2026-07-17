-- w1/m33: typed member-role detail for the team-membership audit verbs —
-- members.ChangeRole records old→new; members.Invite / members.AcceptInvite
-- record the granted role in role_to alone. Same structural guard as
-- plan_from/plan_to (0037): typed nullable columns, no free-form details
-- object, so no secret can reach the feed. Values come from the closed
-- members.Roles ladder.
ALTER TABLE audit_events
    ADD COLUMN role_from text,
    ADD COLUMN role_to text;
