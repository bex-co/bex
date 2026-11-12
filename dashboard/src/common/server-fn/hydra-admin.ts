import { Configuration, OAuth2Api } from "@ory/client-fetch";

/**
 * The Hydra admin client, or null when HYDRA_ADMIN_URL is unset. Shared by the
 * consent, connected-agents and device server functions so every caller
 * resolves the same admin base path (trailing slash trimmed) in one place.
 */
export function hydraAdmin(): OAuth2Api | null {
  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  if (!admin) return null;
  return new OAuth2Api(new Configuration({ basePath: admin }));
}
