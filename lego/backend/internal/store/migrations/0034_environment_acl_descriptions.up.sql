-- w4/m24 migration 0034: environment ip_allow_list gains per-entry
-- descriptions — text[] of CIDRs becomes a jsonb array of
-- {cidrBlock, description} entries (Render's cidrBlockAndDescription wire
-- shape, core.IPAllowListEntry). Existing rows are converted in place to a
-- jsonb array of bare CIDR strings (to_jsonb over the old array) — the legacy
-- serialization core.IPAllowListEntry's UnmarshalJSON accepts with an empty
-- description — so no row is rewritten with fabricated descriptions; the next
-- SetEnvironmentACL write stores full entries.
ALTER TABLE environments
    ALTER COLUMN ip_allow_list DROP DEFAULT,
    ALTER COLUMN ip_allow_list TYPE jsonb USING to_jsonb(ip_allow_list),
    ALTER COLUMN ip_allow_list SET DEFAULT '[]'::jsonb;
