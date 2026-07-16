-- Render service-level binding for private prebuilt images. NULL preserves the
-- legacy host auto-resolution behavior; '' is an explicit no-credential choice;
-- a non-empty value pins the exact registry_credentials id validated by bex-api.
ALTER TABLE apps ADD COLUMN registry_credential_id text;
