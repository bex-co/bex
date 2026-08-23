-- w6/m46 t001: the projector derives spec.expose from the service type, so the
-- row has to carry it. Empty is the historical value and reads as web_service
-- (types.AppSpec); the projector backfills the real type from the live CR.
ALTER TABLE apps ADD COLUMN type text NOT NULL DEFAULT '';
