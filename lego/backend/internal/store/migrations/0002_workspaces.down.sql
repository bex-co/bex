DROP TABLE tenant_members;
ALTER TABLE tenants DROP CONSTRAINT tenants_plan_check;
ALTER TABLE tenants ALTER COLUMN plan SET DEFAULT 'free';
