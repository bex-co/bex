-- w4/089: distinguish a deploy canceled because a newer release superseded it
-- from a user-initiated cancel. User cancel (deploys.Cancel / w6/m52) leaves
-- this empty and keeps failure_reason empty. The reconciler's supersede path
-- stamps a neutral cause here (ideally naming the superseding deploy id) —
-- never into failure_reason, which the dashboard treats as text-destructive.
ALTER TABLE deploys
    ADD COLUMN cancel_reason TEXT NOT NULL DEFAULT '';
