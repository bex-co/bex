/**
 * A login method's name as Render's org member dialog writes it (w1/m657):
 * `pwd` is Password (`password`, translated), anything else its own name
 * capitalized (`google` → Google, `github` → Github).
 */
export function loginMethodLabel(method: string, password: string): string {
  if (method === "pwd") return password;
  return method.charAt(0).toUpperCase() + method.slice(1);
}
