-- Reverse w4/m112's open-deploy stall reason. Dropping it loses only a
-- transient in-flight observation; no terminal deploy state depends on it.
ALTER TABLE deploys DROP COLUMN IF EXISTS stall_reason;
