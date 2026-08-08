-- w1/m65 F2: a GitHub App installation may be connected by at most one
-- workspace. The App JWT can look up every installation of itself, so the
-- connect callback cannot treat a GetInstallation success as ownership proof;
-- enforcing a unique installation->workspace binding at the schema level (plus
-- the application-layer check in internal/github) prevents one tenant claiming
-- another tenant's installation and minting tokens for its repositories.
CREATE UNIQUE INDEX IF NOT EXISTS git_connections_installation_idx
  ON git_connections (installation_id);
