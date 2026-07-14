// bex-native projection of a registry credential row, mapped from bex-api's
// GraphQL RegistryCredential (backend/internal/registrycreds/graphql.go).
// Never carries a secret — the server's read queries don't have one to send.

export interface RegistryCredentialView {
  id: string;
  /** Human display label; defaults to host at creation when left blank. */
  name: string;
  /** The registry hostname this credential authenticates to, e.g. "ghcr.io". */
  host: string;
  username: string;
  /** RFC3339, or null when the credential never expires. */
  expiresAt: string | null;
  /** "active" | "expiring_soon" | "expired" (w2/m14/t007). */
  status: string;
  createdAt: string | null;
}
