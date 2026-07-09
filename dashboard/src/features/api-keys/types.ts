// bex-native projection of bex-api's `ApiKey` (backend/internal/apikeys —
// machine credentials, workspace-shared, not per-user; docs/auth.md). The list
// never carries a secret (the server always resolves it empty for `apiKeys`,
// and the dashboard's list query doesn't even request the field).

export interface ApiKeyView {
  id: string;
  name: string;
  createdAt: string | null;
  /** Identity subject that minted the key (client_id or Kratos id); null if unknown. */
  createdBy: string | null;
  /** RFC 3339 of the last time a token for this key was used; null if never. */
  lastUsedAt: string | null;
}

/** A freshly minted key — the one and only time its secret is available. */
export interface CreatedApiKey {
  id: string;
  name: string;
  secret: string;
}
