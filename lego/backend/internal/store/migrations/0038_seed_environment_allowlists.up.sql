-- w4/m28: environment inbound-IP rules adopt Render's empty-means-deny-all
-- semantics. Before this, an empty ip_allow_list meant OPEN, so every
-- existing empty row is seeded with the explicit allow-all pair — reachability
-- is unchanged and no tenant is locked out by the semantic flip. The ::/0
-- twin is a deliberate bex extension (bex accepts IPv6 CIDRs; an empty list
-- previously projected no middleware at all, leaving IPv6 sources reachable).
UPDATE environments
SET ip_allow_list = '[{"cidrBlock":"0.0.0.0/0","description":"Allow all (migration seed)"},{"cidrBlock":"::/0","description":"Allow all (migration seed)"}]'::jsonb
WHERE ip_allow_list = '[]'::jsonb;
