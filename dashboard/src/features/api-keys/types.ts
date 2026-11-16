// bex-native projection of bex-api's `ApiKey` (backend/internal/apikeys —
// machine credentials, workspace-shared, not per-user; docs/ADR012-auth.md). The list
// never carries the key or its secret (the server always resolves them empty
// for `apiKeys`, and the dashboard's list query doesn't even request the fields).

export interface ApiKeyView {
  id: string;
  name: string;
  createdAt: string | null;
  /** Identity subject that minted the key (client_id or Kratos id); null if unknown. */
  createdBy: string | null;
  /** RFC 3339 of the last time the key was used; null if never. */
  lastUsedAt: string | null;
}

/** A freshly minted key — the one and only time `key` and `secret` are available. */
export interface CreatedApiKey {
  /** The key's id, which is also its OAuth2 client_id. */
  id: string;
  name: string;
  /** The bearer string (`bex_<id>_<secret>`) sent as `Authorization: Bearer`. */
  key: string;
  /** The OAuth2 client_secret, for the optional client_credentials exchange. */
  secret: string;
}
