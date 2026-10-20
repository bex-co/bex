-- w6/m149 (ADR089 § Privilege slice decisions): render-v1 adds exactly
-- Render's measured capability set. Existing rows keep their policy.
ALTER TABLE apps DROP CONSTRAINT apps_container_policy_check;
ALTER TABLE apps ADD CONSTRAINT apps_container_policy_check
  CHECK (container_policy IN ('', 'strict-v1', 'image-v1', 'render-v1'));
