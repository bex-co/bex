/**
 * The login methods bex-api's orgUserAuthStatus lists, in its order
 * (`members.OrgUserAuthStatus`): Render's, without SAML (.pm/DO_NOT_DO.md).
 * View details reserves one row for each while the status loads.
 */
export const LOGIN_METHODS = ["pwd", "google", "github"] as const;

/**
 * A login method's name as Render's org member dialog writes it (w1/m657):
 * `pwd` is Password (`password`, translated), anything else its own name
 * capitalized (`google` → Google, `github` → Github).
 */
export function loginMethodLabel(method: string, password: string): string {
  if (method === "pwd") return password;
  return method.charAt(0).toUpperCase() + method.slice(1);
}
