import { useEffect } from "react";
import { useNavigate, type NavigateOptions } from "@tanstack/react-router";
import { toast } from "sonner";
import { isNotFoundError, translatedText } from "@/common/lib/document-head";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";

/**
 * Has this detail query settled on "there is no such resource"? — the one
 * predicate every detail page feeds `useNotFoundRedirect`.
 *
 * A dead id is not a silent null. bex-api answers `server(id: "<dead>")` with
 * `data.server = null` AND an `errors` entry saying why (`app not found`), so a
 * page that reads a bare `!error` treats every deleted resource as an outage and
 * never redirects. Both halves are needed: settled-and-empty, and the error —
 * if any — is a not-found one.
 *
 * Shared because the four detail pages that hand-rolled `!loading && !x &&
 * !error` all silently lost their w9/m55 redirect the moment the backend started
 * reporting the reason (w6/m44), while `useDeploy`'s copy — which did test the
 * message — kept working. One predicate, one behavior.
 */
export function resourceNotFound(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): boolean {
  return !loading && !resource && (!error || isNotFoundError(error));
}

/**
 * The query settled empty because it FAILED with a genuine outage — not
 * not-found, and not an expired session. An expired session (401) is carved out
 * so it can never render the "check the API and try again" network card (w3/m80
 * t002); `resourceUnauthenticated` claims that case and the retry card stays for
 * real 5xx/transport failures only. Its negation over the settled-and-empty
 * space keeps a page from ever showing two states at once.
 */
export function resourceFailed(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): boolean {
  return (
    !loading &&
    !resource &&
    !!error &&
    !isNotFoundError(error) &&
    !isUnauthenticatedError(error)
  );
}

/**
 * The query settled empty because the session is gone (bex-api answered 401 —
 * see `isUnauthenticatedError`). The client Apollo auth link is already arranging
 * a redirect to login (w3/m80 t001); this predicate lets a detail page render an
 * honest "your session has expired — sign in" state in the meantime instead of a
 * misleading network error. Mutually exclusive with `resourceFailed` and
 * `resourceNotFound` over the settled-and-empty case.
 */
export function resourceUnauthenticated(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): boolean {
  return !loading && !resource && !!error && isUnauthenticatedError(error);
}

/**
 * Which load-error state a settled-empty detail query is in, or null when it's
 * in none: `"unauthenticated"` for an expired session (renders Sign in),
 * `"error"` for a genuine outage (renders Retry). One call at a render site
 * replaces the mutually exclusive `resourceUnauthenticated`/`resourceFailed`
 * pair and maps straight onto `<ResourceLoadError variant>`.
 */
export function resourceLoadErrorVariant(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): "error" | "unauthenticated" | null {
  if (resourceUnauthenticated(resource, loading, error)) {
    return "unauthenticated";
  }
  if (resourceFailed(resource, loading, error)) return "error";
  return null;
}

/**
 * Escape hatch for dead resource ids (w9/m55): once a detail query has settled
 * and the resource is provably absent — never on a failed query, so outages and
 * auth drift keep their inline error UI instead of bouncing users home — replace
 * the dead URL with `target` (default `/`) and toast why. Callers own the
 * error-vs-not-found distinction; pass `notFound` only when the backend
 * resolved the id to nothing. The fixed toast id dedupes StrictMode
 * double-effects and repeated settles.
 */
export function useNotFoundRedirect(
  notFound: boolean,
  target: NavigateOptions = { to: "/" },
) {
  const navigate = useNavigate();
  useEffect(() => {
    if (!notFound) return;
    toast.error(translatedText("common.resourceNotFoundToast"), {
      id: "resource-not-found",
    });
    void navigate({ replace: true, ...target });
  }, [notFound, navigate, target]);
}
