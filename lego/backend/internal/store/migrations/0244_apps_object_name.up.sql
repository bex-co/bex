-- w2/m365: the App object name of a service re-created while a just-deleted
-- service's App still held its CRName(tenant, name) in teardown:
-- "<CRName>-<4 chars>", so the create goes ahead at once, as on Render, while
-- the old App finishes beside it. NULL means CRName(tenant, name). Every path
-- that fetches or projects a service's App reads it (store.App.AppObjectName).
ALTER TABLE apps ADD COLUMN object_name text;
