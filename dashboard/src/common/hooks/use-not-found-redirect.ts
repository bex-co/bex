import { useEffect } from "react";
import { useNavigate, type NavigateOptions } from "@tanstack/react-router";
import { toast } from "sonner";
import { translatedText } from "@/common/lib/document-head";

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
