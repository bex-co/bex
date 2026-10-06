-- The settled rows stay settled: 'running' on a closed deploy was never true.
ALTER TABLE deploys DROP CONSTRAINT IF EXISTS deploys_closed_pre_deploy_settled;
