-- Refuse a downgrade that would leave render-v1 services with a policy the
-- older control plane cannot project.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM apps WHERE container_policy = 'render-v1') THEN
    RAISE EXCEPTION 'cannot remove render-v1 while render-v1 services exist';
  END IF;
END $$;
ALTER TABLE apps DROP CONSTRAINT apps_container_policy_check;
ALTER TABLE apps ADD CONSTRAINT apps_container_policy_check
  CHECK (container_policy IN ('', 'strict-v1', 'image-v1'));
