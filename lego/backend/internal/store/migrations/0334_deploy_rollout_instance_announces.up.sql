-- w32/m575: a native image built since native-init names its start command
-- writes Render's "==> Running '<start command>'" itself, in the instance's
-- own log, at every start. The operator records that on the release's rollout
-- (status.releaseRollout.instanceAnnounces), and the control plane copies it
-- onto the release's deploy row with the rest of the record, so the deploy's
-- log narrates the line only for a release on an older image. NULL, a row an
-- older control plane wrote, reads as false: the narrated line stays.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS rollout_instance_announces boolean;
