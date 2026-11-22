-- w32/m567: Render's deploy log follows "==> Your service is live 🎉" with
-- the URL the service went live at ("==> Available at your primary URL
-- <url>", plus a count of its other domains). The control plane records the
-- URLs a web service's deploy went live at on the deploy's own row, primary
-- first, when it closes the row live, so an older deploy's log keeps its URL
-- after the service gets a new domain. NULL is no record: a row that went
-- live before this column, a row that never went live, and a type with no
-- public URL.
ALTER TABLE deploys ADD COLUMN IF NOT EXISTS live_urls text[];
