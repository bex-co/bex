-- w8/388: Render's port scan names two cases apart from "no open ports": a
-- listener only on localhost ("No open ports detected on 0.0.0.0") and open
-- ports that answer no HTTP ("No open HTTP ports detected on 0.0.0.0"). The
-- control plane records the first of those a running scan found, and when, so
-- the deploy log words each scan line from that moment on as Render does and
-- the lines before it keep their text. NULL is neither.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS port_scan_found text,
  ADD COLUMN IF NOT EXISTS port_scan_found_at timestamptz;
