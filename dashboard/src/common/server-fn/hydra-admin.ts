import { Configuration, OAuth2Api } from "@ory/client-fetch";

/**
 * Hydra's admin API (`HYDRA_ADMIN_URL`, e.g. in-cluster
 * http://hydra-admin.auth.svc:4445), or null when the deployment has not set
 * it. SERVER-ONLY: imported by the OAuth2 server modules (login, consent,
 * connected agents), which their routes load with a dynamic import inside a
 * server handler, so neither the client nor the env reaches the browser.
 */
export function hydraAdmin(): OAuth2Api | null {
  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  if (!admin) return null;
  return new OAuth2Api(new Configuration({ basePath: admin }));
}
