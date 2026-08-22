import { useEffect } from "react";
import { useNavigate, type NavigateOptions } from "@tanstack/react-router";
import { toast } from "sonner";
import { isNotFoundError, translatedText } from "@/common/lib/document-head";

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
 * The complement: the query settled empty because it FAILED, not because the
 * resource is gone. Its exact negation over the settled-and-empty case, so a
 * page can never both redirect home and render its inline retry state — the
 * error-vs-not-found distinction w9/m55 shipped, in one place.
 */
export function resourceFailed(
  resource: unknown,
  loading: boolean,
  error: Error | undefined,
): boolean {
  return !loading && !resource && !!error && !isNotFoundError(error);
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
