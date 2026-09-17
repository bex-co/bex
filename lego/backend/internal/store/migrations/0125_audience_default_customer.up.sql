-- The product-analytics views all expose an `audience` column, but the table
-- backing it was an opt-in override that almost nobody opted into: on
-- 2026-09-16, 1 of 31 workspaces carried a row and everything else collapsed
-- into COALESCE(...,'unclassified'). A dimension that is 97% "unknown" cannot
-- support a grouped read, so QA churn and customer activity were indistinguishable
-- in every series.
--
-- Two changes make the dimension usable and keep it that way:
--   1. backfill every existing workspace as 'customer' (the overwhelmingly common
--      case; non-customer workspaces are the exception and get reclassified), and
--   2. a trigger so a newly created workspace is classified at birth. A trigger
--      rather than Go call sites because tenants are created down three separate
--      paths (CreateTenant, CreateTenantWithMember, FinalizeWorkspaceCreation) and
--      any future fourth would silently reopen the hole.
--
-- Reclassification stays a plain UPDATE; the trigger only ever supplies a default.
INSERT INTO product_analytics_audiences (workspace_id, audience)
SELECT t.id, 'customer' FROM tenants t
ON CONFLICT (workspace_id) DO NOTHING;

CREATE OR REPLACE FUNCTION product_analytics_default_audience() RETURNS trigger AS $$
BEGIN
    INSERT INTO product_analytics_audiences (workspace_id, audience)
    VALUES (NEW.id, 'customer')
    ON CONFLICT (workspace_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tenants_default_audience ON tenants;
CREATE TRIGGER tenants_default_audience
    AFTER INSERT ON tenants
    FOR EACH ROW EXECUTE FUNCTION product_analytics_default_audience();
