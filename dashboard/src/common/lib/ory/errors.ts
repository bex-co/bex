/**
 * Kratos answers a refused call with a machine-readable id — `session_aal2_required`,
 * `session_already_available`, `browser_location_change_required`, … — and that id,
 * not the HTTP status, is what tells the caller what to do next. The SDK throws the
 * raw `Response`, so every caller has to dig the id out of the body; this is the one
 * place that knows how.
 *
 * `redirect_browser_to` rides along because Kratos pairs it with the errors that mean
 * "this flow must continue somewhere else".
 *
 * Read via `clone()`: the body is single-use, and a caller may well look at the same
 * error twice (a retry cascade re-inspects the error it caught).
 */
export async function oryErrorInfo(
  err: unknown,
): Promise<{ id?: string; redirectBrowserTo?: string }> {
  const response = (err as { response?: Response })?.response;
  if (!response) return {};
  try {
    const body = (await response.clone().json()) as {
      error?: { id?: string };
      redirect_browser_to?: string;
    };
    return { id: body.error?.id, redirectBrowserTo: body.redirect_browser_to };
  } catch {
    return {};
  }
}
